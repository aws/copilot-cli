// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

const AWS = require('aws-sdk');
let ENV_HOSTED_ZONE_ID;
let APP_NAME, ENV_NAME;

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

    let loadBalancerDNS = props.LoadBalancerDNS;
    let aliases = props.Aliases;
    let certDomain = `${props.EnvName}.${props.AppName}.${props.DomainName}`;

    const physicalResourceID = `/${serviceName}/${customDomain}`; // sorted default nlb alias and custom aliases;

    switch (event.RequestType) {
        case "Create":
            await validateAliases(aliases, loadBalancerDNS);
            requestCertificate();
            waitForValidationOptionsToBeReady();
            validateCertAndAddAliases();
            break;
        case "Update":
            // check if certificate should update; if yes, replace; if not, exit
            await validateAliases(aliases);
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
 * @param {Array<String>} aliases for the service.
 * @param {String} loadBalancerDNS the DNS of the service's load balancer.
 *
 * @throws error if at least one of the aliases is not valid.
 */
async function validateAliases(aliases, loadBalancerDNS) {
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
        if (recordSet[0].AliasTarget && recordSet[0].AliasTarget.DNSName === `${loadBalancerDNS}.`) {
            continue; // The record is an alias record and is in use by myself, hence valid.
        }
        throw new Error(`alias ${alias} is in use`);
    }
}

/**
 * Requests a public certificate from AWS Certificate Manager, using DNS validation.
 * @param {String} certDomain the certificate domain.
 * @param {Array<String>} aliases the subject alternative names for the certificate.
 *
 * @return {String} The ARN of the requested certificate.
 */
async function requestCertificate(certDomain, aliases) {
    const acm = new AWS.ACM();
    const data =await acm.requestCertificate({
        DomainName: certDomain,
        IdempotencyToken: "1", // TODO: this should be the physical resource id
        SubjectAlternativeNames: aliases,
        Tags: [
            {
                Key: "copilot-application",
                Value: APP_NAME,
            },
            {
                Key: "copilot-environment",
                Value: ENV_NAME,
            },
        ],
        ValidationMethod: "DNS",
    }).promise();
    return data["CertificateArn"];
}