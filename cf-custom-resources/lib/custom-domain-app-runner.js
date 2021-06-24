// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/* jshint node: true */
/*jshint esversion: 8 */

"use strict";

const AWS = require('aws-sdk');
let appRoute53Client, appRunnerClient, appHostedZoneID;

const { sendResponse } = require('../lib/partials');

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

    await addCustomDomain(serviceARN, customDomain).then(async () => {
        await sendResponse(event, context, "SUCCESS", event.LogicalResourceId);
    }).catch(async err => {
        await sendResponse(event, context, "FAILED", event.LogicalResourceId, null, err.message).catch((err) => {
            throw new Error("send response: " + err.message);
        });
    });
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
    const data = await appRunnerClient.describeCustomDomains({
        ServiceArn: serviceARN,
    }).promise().catch((err) => {
        throw new Error(`get custom domains for service ${serviceARN}: ` + err.message);
    });

    const customDomains = data.CustomDomains;
    for (const i in customDomains) {
        if (customDomains[i].DomainName !== domainName) {
            continue;
        }
        const records = customDomains[i].CertificateValidationRecords;
        for (const i in records) {
            await upsertCNAMERecordAndWait(records[i].Name, records[i].Value, appHostedZoneID).catch(err => {
                throw new Error("upsert certificate validation record: " + err.message);
            });
        }
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