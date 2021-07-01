// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/* jshint node: true */
/*jshint esversion: 8 */

"use strict";

const AWS = require('aws-sdk');
const { report, defaultSleep } = require('../lib/partials');

const ERR_NAME_CUSTOM_DOMAIN_ALREADY_ASSOCIATED = "CustomDomainAlreadyAssociatedException";
const ERR_NAME_INVALID_REQUEST = "InvalidRequestException";
const DOMAIN_STATUS_PENDING_VERIFICATION = "pending_certificate_dns_validation";
const ATTEMPTS = 10;


let sleep = defaultSleep;
let appRoute53Client, appRunnerClient, appHostedZoneID;

exports.handler = async function (event, context) {
    const props = event.ResourceProperties;
    const [serviceARN, appDNSRole, customDomain] = [props.ServiceARN, props.AppDNSRole, props.CustomDomain,];
    appHostedZoneID = props.HostedZoneID;

    // Configure clients.
    appRoute53Client = new AWS.Route53({
        credentials: new AWS.ChainableTemporaryCredentials({
            params: { RoleArn: appDNSRole, },
            masterCredentials: new AWS.EnvironmentCredentials("AWS"),
        }),
    });
    appRunnerClient = new AWS.AppRunner();

    let addCustomDomainErr;
    await addCustomDomain(serviceARN, customDomain).catch(async err => {
        addCustomDomainErr = err;
        if (err.name === "CustomDomainAlreadyAssociated") {
            console.log("Custom domain already associated. Do nothing.");
            return;
        }
        console.log(`Caught error: ${err.message}`);
        await report(event, context, "FAILED", event.LogicalResourceId, null, err.message).catch((err) => {
            throw new Error("send response: " + err.message);
        });
    });

    if (!addCustomDomainErr) {
        console.log("Finished");
        await report(event, context, "SUCCESS", event.LogicalResourceId).catch((err) => {
            throw new Error("send response: " + err.message);
        });
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
    }).promise().catch(err => {
        if (err.name === ERR_NAME_INVALID_REQUEST && err.message.includes(`${customDomainName} is already associated with`)) {
            throw new CustomDomainError(`${customDomainName} is already associated with service ${serviceARN}`, ERR_NAME_CUSTOM_DOMAIN_ALREADY_ASSOCIATED);
        }
        throw err;
    });

    await upsertCNAMERecordAndWait(customDomainName, data.DNSTarget, appHostedZoneID);
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
            await upsertCNAMERecordAndWait(records[i].Name, records[i].Value, appHostedZoneID).catch(err => {
                throw new Error("upsert certificate validation record: " + err.message);
            });
        }
        break;
    }

    if (i >= ATTEMPTS) {
        throw new Error(`failed waiting for custom domain ${domainName} to change to state ${DOMAIN_STATUS_PENDING_VERIFICATION}`);
    }
}

/**
 * Upserts a CNAME record and wait for the change to have taken place.
 *
 * @param {string} recordName the name of the record
 * @param {string} recordValue the value of the record
 * @param {string} hostedZoneID the ID of the hosted zone into which the record needs to be upserted.
 * @throws wrapped error.
 */
async function upsertCNAMERecordAndWait(recordName, recordValue, hostedZoneID) {
    console.log(`Upsert record ${recordName}`);
    let params = {
        ChangeBatch: {
            Changes: [
                {
                    Action: "UPSERT",
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

function CustomDomainError(message, name) {
    this.name = name;
    this.message = message;
    this.stack = (new Error()).stack;
}
CustomDomainError.prototype = Object.create(Error.prototype);

exports.domainStatusPendingVerification = DOMAIN_STATUS_PENDING_VERIFICATION;
exports.waitForDomainStatusChangeAttempts = ATTEMPTS;
exports.withSleep = function (s) {
    sleep = s;
};
exports.reset = function () {
    sleep = defaultSleep;
};