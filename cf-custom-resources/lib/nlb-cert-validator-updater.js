// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

const AWS = require('aws-sdk');
let ENV_HOSTED_ZONE_ID;
let MY_LOAD_BALANCER_DNS;

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
    ENV_HOSTED_ZONE_ID = props.EnvHostedZoneId;
    MY_LOAD_BALANCER_DNS = props.LoadBalancerDNS;

    switch (event.RequestType) {
        case "Create":
            validateAliases(); // check if it is in used
            requestCertificate();
            waitForValidationOptionsToBeReady();
            validateCertAndAddAliases();
            break;
        case "Update":
            // check if certificate should update; if yes, replace; if not, exit
            validateAliases(); // check if it is in used
            requestCertificate();
            waitForValidationOptionsToBeReady();
            validateCertAndAddAliases(); // for each alias
            break;
        case "Delete":
            deleteCertificate();
            deleteValidationRecordsAndAliases(); // for each alias, also check if the alias points to myself before deleting.
            break;
        default:
            throw new Error(`Unsupported request type ${event.RequestType}`);
    }
};

/**
 * Validate that the aliases are not in use.
 */
async function validateAliases(aliases) {
    const envRoute53 = new AWS.Route53();
    for (let alias of aliases) {
        const data = await envRoute53.listResourceRecordSets({
            HostedZoneId: ENV_HOSTED_ZONE_ID,
            MaxItems: "1",
            StartRecordName: alias,
        }).promise();

        let recordSet = data["ResourceRecordSets"];
        if (!recordSet || recordSet.length === 0) {
            continue; // The alias record does not exist in the hosted zone, hence valid.
        }
        if (recordSet[0].AliasTarget && recordSet[0].AliasTarget.DNSName === `${MY_LOAD_BALANCER_DNS}.`) {
            continue; // The record is an alias record and is in use by myself, hence valid.
        }
        return false;
    }
    return true;
}