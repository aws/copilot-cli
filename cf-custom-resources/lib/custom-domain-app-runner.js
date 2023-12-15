// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/* jshint node: true */
/*jshint esversion: 8 */

"use strict";

const AWS = require('aws-sdk');

const DOMAIN_STATUS_PENDING_VERIFICATION = "pending_certificate_dns_validation";
const DOMAIN_STATUS_ACTIVE = "active";
const DOMAIN_STATUS_DELETE_FAILED = "delete_failed";
const ATTEMPTS_WAIT_FOR_PENDING = 10;
// Expectedly lambda time out would be triggered before 20-th attempt. This ensures that we attempts to wait for it to be disassociated as much as possible.
const ATTEMPTS_WAIT_FOR_DISASSOCIATED = 20;

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

        let reasonWithLogInfo = `${reason} (Log: ${context.logGroupName}/${context.logStreamName})`;
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
    const [serviceARN, appDNSRole, customDomain, appDNSName] = [props.ServiceARN, props.AppDNSRole, props.CustomDomain, props.AppDNSName, ];
    const physicalResourceID = `/associate-domain-app-runner/${customDomain}`;
    let handler = async function () {
        // Configure clients.
        appRoute53Client = new AWS.Route53({
            credentials: new AWS.ChainableTemporaryCredentials({
                params: { RoleArn: appDNSRole, },
                masterCredentials: new AWS.EnvironmentCredentials("AWS"),
            }),
        });
        appRunnerClient = new AWS.AppRunner();
        appHostedZoneID = await domainHostedZoneID(appDNSName);
        switch (event.RequestType) {
            case "Create":
            case "Update":
                await addCustomDomain(serviceARN, customDomain);
                break;
            case "Delete":
                await removeCustomDomain(serviceARN, customDomain);
                await waitForCustomDomainToBeDisassociated(serviceARN, customDomain);
                break;
            default:
                throw new Error(`Unsupported request type ${event.RequestType}`);
        }
    };
    try {
        await Promise.race([exports.deadlineExpired(), handler(),]);
        await report(event, context, "SUCCESS", physicalResourceID);
    } catch (err) {
        console.log(`Caught error for service ${serviceARN}: ${err.message}`);
        await report(event, context, "FAILED", physicalResourceID, null, err.message);
    }
};

exports.deadlineExpired = function () {
    return new Promise(function (resolve, reject) {
        setTimeout(
            reject,
            14 * 60 * 1000 + 30 * 1000 /* 14.5 minutes*/,
            new Error(`Lambda took longer than 14.5 minutes to update custom domain`)
        );
    });
};

/**
 * Get the hosted zone ID of the domain name from the app account.
 * @param {string} domainName
 */
async function domainHostedZoneID(domainName) {
    const data = await appRoute53Client.listHostedZonesByName({
        DNSName: domainName,
        MaxItems: "1",
    }).promise();

    if (!data.HostedZones || data.HostedZones.length === 0) {
        throw new Error(`couldn't find any Hosted Zone with DNS name ${domainName}`);
    }
    return data.HostedZones[0].Id.split("/").pop();
}

/**
 * Add custom domain for service by associating and adding records for both the domain and the validation.
 * Errors are not handled and are directly passed to the caller.
 *
 * @param {string} serviceARN ARN of the service that the custom domain applies to.
 * @param {string} customDomainName the custom domain name.
 */
async function addCustomDomain(serviceARN, customDomainName) {
    let data;
    try {
        data = await appRunnerClient.associateCustomDomain({
            DomainName: customDomainName,
            ServiceArn: serviceARN,
        }).promise();
    } catch (err) {
        const isDomainAlreadyAssociated = err.message.includes(`${customDomainName} is already associated with`);
        if (!isDomainAlreadyAssociated) {
            throw err;
        }
    }

    if (!data) {
        // If domain is already associated, data would be undefined.
        data = await appRunnerClient.describeCustomDomains({
            ServiceArn: serviceARN,
        }).promise();
    }

    return Promise.all([
        updateCNAMERecordAndWait(customDomainName, data.DNSTarget, appHostedZoneID, "UPSERT"), // Upsert the record that maps `customDomainName` to the DNS of the app runner service.
        validateCertForDomain(serviceARN, customDomainName),
    ]);
}

/**
 * Get information about domain.
 * @param {string} serviceARN
 * @param {string} domainName
 * @returns {object} CustomDomain object that contains information such as DomainName, Status, CertificateValidationRecords, etc.
 * @throws error if domain is not found in service.
 */
async function getDomainInfo(serviceARN, domainName) {
    let describeCustomDomainsInput = {ServiceArn: serviceARN,};
    while (true) {
        const resp = await appRunnerClient.describeCustomDomains(describeCustomDomainsInput).promise();

        for (const d of resp.CustomDomains) {
            if (d.DomainName === domainName) {
                return d;
            }
        }

        if (!resp.NextToken) {
            throw new NotAssociatedError(`domain ${domainName} is not associated`);
        }
        describeCustomDomainsInput.NextToken = resp.NextToken;
    }
}

/**
 * Validate certificates of the custom domain for the service by upserting validation records.
 *
 * @param {string} serviceARN ARN of the service that the custom domain applies to.
 * @param {string} domainName the custom domain name.
 * @throws wrapped error.
 */
async function validateCertForDomain(serviceARN, domainName) {
    let i, lastDomainStatus;
    for (i = 0; i < ATTEMPTS_WAIT_FOR_PENDING; i++){

        const domain = await getDomainInfo(serviceARN, domainName).catch(err => {
            throw new Error(`update validation records for domain ${domainName}: ` + err.message);
        });

        lastDomainStatus = domain.Status;

        if (!domainValidationRecordReady(domain)) {
            await sleep(3000);
            continue;
        }

        // Upsert all records needed for certificate validation.
        const records = domain.CertificateValidationRecords;
        let promises = [];
        for (const record of records) {
            promises.push(
                updateCNAMERecordAndWait(record.Name, record.Value, appHostedZoneID, "UPSERT").catch(err => {
                    throw new Error(`update validation records for domain ${domainName}: ` + err.message);
                })
            );
        }
        return Promise.all(promises);
    }

    if (i === ATTEMPTS_WAIT_FOR_PENDING) {
        throw new Error(`update validation records for domain ${domainName}: fail to wait for state ${DOMAIN_STATUS_PENDING_VERIFICATION}, stuck in ${lastDomainStatus}`);
    }
}

/**
 * There are one known scenarios where status could be ACTIVE right after it's associated:
 * When the domain just got deleted and added again. In this case, even though the validation records could
 * have been deleted, the previously successful validation results are still cached. Because of the cache,
 * the domain will show to be ACTIVE immediately after it's associated , although the validation records are not
 * there anymore.
 * In this case, the status won't transit to PENDING_VERIFICATION, so we need to check whether the validation
 * records are ready by counting if there are three of them.
 *
 * @param {string} domain
 * @returns {boolean}
 */
function domainValidationRecordReady(domain) {
    if (domain.Status === DOMAIN_STATUS_PENDING_VERIFICATION) {
        return true;
    }

    if (domain.Status === DOMAIN_STATUS_ACTIVE && domain.CertificateValidationRecords && domain.CertificateValidationRecords.length === 3) {
        return true;
    }

    return false;
}

/**
 * Remove custom domain from service by disassociating and removing the records for both the domain and the validation.
 * If the custom domain is not found in the service, the function returns without error.
 * Errors are not handled and are directly passed to the caller.
 *
 * @param {string} serviceARN ARN of the service that the custom domain applies to.
 * @param {string} customDomainName the custom domain name.
 */
async function removeCustomDomain(serviceARN, customDomainName) {
    let data;
    try {
        data = await appRunnerClient.disassociateCustomDomain({
            DomainName: customDomainName,
            ServiceArn: serviceARN,
        }).promise();
    } catch (err) {
        if (err.message.includes(`No custom domain ${customDomainName} found for the provided service`)) {
            return;
        }
        throw err;
    }

    return Promise.all([
        updateCNAMERecordAndWait(customDomainName, data.DNSTarget, appHostedZoneID, "DELETE"), // Delete the record that maps `customDomainName` to the DNS of the app runner service.
        removeValidationRecords(data.CustomDomain),
    ]);
}

/**
 * Remove validation records for a custom domain.
 *
 * @param {object} domain information containing DomainName, Status, CertificateValidationRecords, etc.
 * @throws wrapped error.
 */
async function removeValidationRecords(domain) {
    const records = domain.CertificateValidationRecords;
    let promises = [];
    for (const record of records) {
        promises.push(
            updateCNAMERecordAndWait(record.Name, record.Value, appHostedZoneID, "DELETE").catch(err => {
                throw new Error(`delete validation records for domain ${domain.DomainName}: ` + err.message);
            })
        );
    }
    return Promise.all(promises);
}

/**
 * Wait for the custom domain to be disassociated.
 * @param {string} serviceARN the service to which the domain is added.
 * @param {string} customDomainName the domain name.
 */
async function waitForCustomDomainToBeDisassociated(serviceARN, customDomainName) {
    let lastDomainStatus;
    for (let i = 0; i < ATTEMPTS_WAIT_FOR_DISASSOCIATED; i++) {
        let domain;
        try {
            domain = await getDomainInfo(serviceARN, customDomainName);
        } catch (err) {
            // Domain is disassociated.
            if (err instanceof NotAssociatedError) {
                return;
            }
            throw new Error(`wait for domain ${customDomainName} to be unused: ` + err.message);
        }

        lastDomainStatus = domain.Status;

        if (lastDomainStatus === DOMAIN_STATUS_DELETE_FAILED) {
            throw new Error(`fail to disassociate domain ${customDomainName}: domain status is ${DOMAIN_STATUS_DELETE_FAILED}`);
        }

        const base = Math.pow(2, i);
        await sleep(Math.random() * base * 50 + base * 150);
    }

    console.log(`Fail to wait for the domain status to be disassociated. The last reported status of domain ${customDomainName} is ${lastDomainStatus}`);
    throw new Error(`fail to wait for domain ${customDomainName} to be disassociated`);
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

    let data;
    try {
        data = await appRoute53Client.changeResourceRecordSets(params).promise();
    } catch (err) {
        let recordSetNotFoundErrMessageRegex = /Tried to delete resource record set \[name='.*', type='CNAME'] but it was not found/;
        if (action === "DELETE" && err.message.search(recordSetNotFoundErrMessageRegex) !== -1) {
            return; // If we attempt to `DELETE` a record that doesn't exist, the job is already done, skip waiting.
        }
        throw new Error(`update record ${recordName}: ` + err.message);
    }

    await appRoute53Client.waitFor('resourceRecordSetsChanged', {
         // Wait up to 5 minutes
         $waiter: {
             delay: 30,
             maxAttempts: 10,
         },
         Id: data.ChangeInfo.Id,
     }).promise().catch((err) => {
         throw new Error(`update record ${recordName}: wait for record sets change for ${recordName}: ` + err.message);
     });
}

function NotAssociatedError(message = "") {
    this.message = message;
}
NotAssociatedError.prototype = Error.prototype;

exports.domainStatusPendingVerification = DOMAIN_STATUS_PENDING_VERIFICATION;
exports.waitForDomainStatusPendingAttempts = ATTEMPTS_WAIT_FOR_PENDING;
exports.waitForDomainToBeDisassociatedAttempts = ATTEMPTS_WAIT_FOR_DISASSOCIATED;
exports.withSleep = function (s) {
    sleep = s;
};
exports.reset = function () {
    sleep = defaultSleep;
};
exports.withDeadlineExpired = function (d) {
    exports.deadlineExpired = d;
};
