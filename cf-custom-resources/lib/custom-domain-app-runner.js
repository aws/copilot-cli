// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/* jshint node: true */
/*jshint esversion: 8 */

"use strict";

const AWS = require('aws-sdk');

const ERR_NAME_INVALID_REQUEST = "InvalidRequestException";
const DOMAIN_STATUS_PENDING_VERIFICATION = "pending_certificate_dns_validation";
const ATTEMPTS = 10;

let defaultSleep = function (ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
};
let sleep = defaultSleep;
let appRoute53Client, appRunnerClient, appHostedZoneID;

/**
 * Upload a CloudFormation response object to S3.
 *
 * @param {object} event the Lambda event payload received by the handler function
 * @param {object} context the Lambda context received by the handler function
 * @param {string} responseStatus the response status, either 'SUCCESS' or 'FAILED'
 * @param {string} physicalResourceId CloudFormation physical resource ID
 * @param {object} [responseData] arbitrary response data object
 * @param {string} [reason] reason for failure, if any, to convey to the user
 * @returns {Promise} Promise that is resolved on success, or rejected on connection error or HTTP error response
 */
function report (
    event,
    context,
    responseStatus,
    physicalResourceId,
    responseData,
    reason
) {
    return new Promise((resolve, reject) => {
        const https = require("https");
        const { URL } = require("url");

        let reasonWithLogInfo = `${reason} (Log: ${context.logGroupName}${context.logStreamName})`;
        var responseBody = JSON.stringify({
            Status: responseStatus,
            Reason: reasonWithLogInfo,
            PhysicalResourceId: physicalResourceId || context.logStreamName,
            StackId: event.StackId,
            RequestId: event.RequestId,
            LogicalResourceId: event.LogicalResourceId,
            Data: responseData,
        });

        const parsedUrl = new URL(event.ResponseURL);
        const options = {
            hostname: parsedUrl.hostname,
            port: 443,
            path: parsedUrl.pathname + parsedUrl.search,
            method: "PUT",
            headers: {
                "Content-Type": "",
                "Content-Length": responseBody.length,
            },
        };

        https
            .request(options)
            .on("error", reject)
            .on("response", (res) => {
                res.resume();
                if (res.statusCode >= 400) {
                    reject(new Error(`Error ${res.statusCode}: ${res.statusMessage}`));
                } else {
                    resolve();
                }
            })
            .end(responseBody, "utf8");
    });
}

exports.handler = async function (event, context) {
    const props = event.ResourceProperties;
    const [serviceARN, appDNSRole, customDomain] = [props.ServiceARN, props.AppDNSRole, props.CustomDomain,];
    appHostedZoneID = props.HostedZoneID;
    try {
        // Configure clients.
        appRoute53Client = new AWS.Route53({
            credentials: new AWS.ChainableTemporaryCredentials({
                params: { RoleArn: appDNSRole, },
                masterCredentials: new AWS.EnvironmentCredentials("AWS"),
            }),
        });
        appRunnerClient = new AWS.AppRunner();

        switch (event.RequestType) {
            case "Create":
                await addCustomDomain(serviceARN, customDomain);
                console.log("Finished");
                break;
            case "Update":
            case "Delete":
                throw new Error("not yet implemented");
            default:
                throw new Error(`Unsupported request type ${event.RequestType}`);
        }
        await report(event, context, "SUCCESS", event.LogicalResourceId);
    } catch (err) {
        if (err.name === ERR_NAME_INVALID_REQUEST && err.message.includes(`${customDomain} is already associated with`)) {
            console.log("Custom domain already associated. Do nothing.");
            await report(event, context, "SUCCESS", event.LogicalResourceId);
            return;
        }
        console.log(`Caught error: ${err.message}`);
        await report(event, context, "FAILED", event.LogicalResourceId, null, err.message);
    }
};

/**
 * Validate certificates of the custom domain for the service by upserting validation records.
 * Errors are not handled and are directly passed to the caller.
 *
 * @param {string} serviceARN ARN of the service that the custom domain applies to.
 * @param {string} customDomainName the custom domain name.
 */
async function addCustomDomain(serviceARN, customDomainName) {
    const data = await appRunnerClient.associateCustomDomain({
        DomainName: customDomainName,
        ServiceArn: serviceARN,
    }).promise();

    await updateCNAMERecordAndWait(customDomainName, data.DNSTarget, appHostedZoneID, "UPSERT");
    await validateCertForDomain(serviceARN, customDomainName);
}

/**
 * Validate certificates of the custom domain for the service by upserting validation records.
 *
 * @param {string} serviceARN ARN of the service that the custom domain applies to.
 * @param {string} domainName the custom domain name.
 * @throws wrapped error.
 */
async function validateCertForDomain(serviceARN, domainName) {
    console.log("Add validation records");
    let i;
    for (i = 0; i < ATTEMPTS; i++){
        const data = await appRunnerClient.describeCustomDomains({
            ServiceArn: serviceARN,
        }).promise().catch(err => {
            throw new Error(`get custom domains for service ${serviceARN}: ` + err.message);
        });

        const customDomains = data.CustomDomains;
        let domain;
        for (const i in customDomains) {
            if (customDomains[i].DomainName === domainName) {
                domain = customDomains[i];
                break;
            }
        }
        if (!domain) {
            throw new Error(`domain ${domainName} is not associated`);
        }

        if (domain.Status !== DOMAIN_STATUS_PENDING_VERIFICATION) {
            console.log(`Custom domain ${domainName} status is ${domain.Status}. Desired state is ${DOMAIN_STATUS_PENDING_VERIFICATION}. Wait and check again`);
            await sleep(3000);
            continue;
        }

        const records = domain.CertificateValidationRecords;
        for (const i in records) {
            await updateCNAMERecordAndWait(records[i].Name, records[i].Value, appHostedZoneID, "UPSERT").catch(err => {
                throw new Error("upsert certificate validation record: " + err.message);
            });
        }
        break;
    }

    if (i === ATTEMPTS) {
        throw new Error(`failed waiting for custom domain ${domainName} to change to state ${DOMAIN_STATUS_PENDING_VERIFICATION}`);
    }
}

/**
 * Upserts a CNAME record and wait for the change to have taken place.
 *
 * @param {string} recordName the name of the record
 * @param {string} recordValue the value of the record
 * @param {string} hostedZoneID the ID of the hosted zone into which the record needs to be upserted.
 * @param {string} action the action to perform; can be "CREATE", "DELETE", or "UPSERT".
 * @throws wrapped error.
 */
async function updateCNAMERecordAndWait(recordName, recordValue, hostedZoneID, action) {
    console.log(`Upsert record ${recordName}`);
    let params = {
        ChangeBatch: {
            Changes: [
                {
                    Action: action,
                    ResourceRecordSet: {
                        Name: recordName,
                        Type: "CNAME",
                        TTL: 60,
                        ResourceRecords: [
                            {
                                Value: recordValue,
                            },
                        ],
                    },
                },
            ],
        },
        HostedZoneId: hostedZoneID,
    };

     const data = await appRoute53Client.changeResourceRecordSets(params).promise().catch((err) => {
        throw new Error(`upsert record ${recordName}: ` + err.message);
    });

     console.log("Finished upserting, start waiting");
     await appRoute53Client.waitFor('resourceRecordSetsChanged', {
         // Wait up to 5 minutes
         $waiter: {
             delay: 30,
             maxAttempts: 10,
         },
         Id: data.ChangeInfo.Id,
     }).promise().catch((err) => {
         throw new Error(`wait for record sets change for ${recordName}: ` + err.message);
     });
}

exports.domainStatusPendingVerification = DOMAIN_STATUS_PENDING_VERIFICATION;
exports.waitForDomainStatusChangeAttempts = ATTEMPTS;
exports.withSleep = function (s) {
    sleep = s;
};
exports.reset = function () {
    sleep = defaultSleep;
};