// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

const AWS = require('aws-sdk');
const ATTEMPTS_VALIDATION_OPTIONS_READY = 10;
const ATTEMPTS_RECORD_SETS_CHANGE = 10;
const DELAY_RECORD_SETS_CHANGE = 30;
const ATTEMPTS_CERTIFICATE_VALIDATED = 19;
const DELAY_CERTIFICATE_VALIDATED = 30;

let acm, envRoute53, envHostedZoneID, appName, envName, certificateDomain;
let defaultSleep = function (ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
};
let sleep = defaultSleep;
let random = Math.random;

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
    acm = new AWS.ACM();
    envRoute53 = new AWS.Route53();

    const props = event.ResourceProperties;
    envHostedZoneID = props.EnvHostedZoneId;
    certificateDomain = `${props.ServiceName}-nlb.${props.EnvName}.${props.AppName}.${props.DomainName}`;

    let loadBalancerDNS = props.LoadBalancerDNS;
    let loadBalancerHostedZoneID = props.LoadBalancerHostedZoneID;
    let aliases = new Set(props.Aliases);

    const physicalResourceID = `/${serviceName}/${customDomain}`; // sorted default nlb alias and custom aliases;

    switch (event.RequestType) {
        case "Create":
            await validateAliases(aliases, loadBalancerDNS);
            const certificateARN = await requestCertificate(aliases);
            const options = await waitForValidationOptionsToBeReady(certificateARN, aliases);
            await activate(options, certificateARN, loadBalancerDNS, loadBalancerHostedZoneID);
            break;
        case "Update":
            // check if certificate should update; if yes, replace; if not, exit
            // await validateAliases(aliases);
            // requestCertificate();
            // waitForValidationOptionsToBeReady();
            // validateCertAndAddAliases(); // for each alias
            break;
        case "Delete":
            // deleteCertificate();
            // deleteValidationRecordsAndAliases(); // for each alias, also check if the alias points to myself before deleting.
            break;
        default:
            throw new Error(`Unsupported request type ${event.RequestType}`);
    }
};

/**
 * Validate that the aliases are not in use.
 *
 * @param {Set<String>} aliases for the service.
 * @param {String} loadBalancerDNS the DNS of the service's load balancer.
 * @throws error if at least one of the aliases is not valid.
 */
async function validateAliases(aliases, loadBalancerDNS) {
    for (let alias of aliases) {
        const data = await envRoute53.listResourceRecordSets({
            HostedZoneId: envHostedZoneID,
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
 *
 * @param {Set<String>} aliases the subject alternative names for the certificate.
 * @return {String} The ARN of the requested certificate.
 */
async function requestCertificate(aliases) {
    const { CertificateArn } =await acm.requestCertificate({
        DomainName: certificateDomain,
        IdempotencyToken: "1", // TODO: this should be the physical resource id
        SubjectAlternativeNames: aliases.size === 0? null: [...aliases],
        Tags: [
            {
                Key: "copilot-application",
                Value: appName,
            },
            {
                Key: "copilot-environment",
                Value: envName,
            },
        ],
        ValidationMethod: "DNS",
    }).promise();
    return CertificateArn;
}

/**
 * Wait until the validation options are ready
 *
 * @param certificateARN
 * @param {Set<String>} aliases for the service.
 */
async function waitForValidationOptionsToBeReady(certificateARN, aliases) {
    let expectedCount = aliases.size + 1; // Expect one validation option for each alias and the cert domain.

    let attempt; // TODO: This wait loops could be further abstracted.
    for (attempt = 0; attempt < ATTEMPTS_VALIDATION_OPTIONS_READY; attempt++) {
        let readyCount = 0;
        const { Certificate } = await acm.describeCertificate({
            CertificateArn: certificateARN,
        }).promise();
        const options = Certificate.DomainValidationOptions || [];
        options.forEach(option => {
            if (option.ResourceRecord && (aliases.has(option.DomainName) || option.DomainName === certificateDomain)) {
                readyCount++;
            }
        })
        if (readyCount === expectedCount) {
            return options;
        }

        // Exponential backoff with jitter based on 200ms base
        // component of backoff fixed to ensure minimum total wait time on
        // slow targets.
        const base = Math.pow(2, attempt);
        await sleep(random() * base * 50 + base * 150);
    }
    throw new Error(`resource validation records are not ready after ${attempt} tries`);
}

/**
 * Validate the certificate and insert the alias records
 *
 * @param {Array<Object>} validationOptions
 * @param {String} certificateARN
 * @param {String} loadBalancerDNS
 * @param {String} loadBalancerHostedZone
 */
async function activate(validationOptions, certificateARN, loadBalancerDNS, loadBalancerHostedZone) {
    let promises = [];
    for (let option of validationOptions) {
        promises.push(activateOption(option, loadBalancerDNS, loadBalancerHostedZone));
    }
    await Promise.all(promises);
    console.log("finished upserting records");

    await acm.waitFor("certificateValidated", {
        // Wait up to 9 minutes and 30 seconds
        $waiter: {
            delay: DELAY_CERTIFICATE_VALIDATED,
            maxAttempts: ATTEMPTS_CERTIFICATE_VALIDATED,
        },
        CertificateArn: certificateARN,
    }).promise();
}

/**
 * Upsert the validation record for the alias, as well as adding the A record if the alias is not the default certificaite domain.
 *
 * @param {Array<object>} option
 * @param {String} loadBalancerDNS
 * @param {String} loadBalancerHostedZone
 */
async function activateOption(option, loadBalancerDNS, loadBalancerHostedZone) {
    let changes = [{
        Action: "UPSERT",
        ResourceRecordSet: {
            Name: option.ResourceRecord.Name,
            Type: option.ResourceRecord.Type,
            TTL: 60,
            ResourceRecords: [
                {
                    Value: option.ResourceRecord.Value,
                },
            ],
        }
    }];

    if (option.DomainName !== certificateDomain) {
        changes.push({
            Action: "UPSERT", // It is validated that if the alias is in use, it is in use by the service itself.
            ResourceRecordSet: {
                Name: option.DomainName,
                Type: "A",
                AliasTarget: {
                    DNSName: loadBalancerDNS,
                    EvaluateTargetHealth: true,
                    HostedZoneId: loadBalancerHostedZone,
                }
            }
        });
    }

    let { ChangeInfo } = await envRoute53.changeResourceRecordSets({
        ChangeBatch: {
            Comment: "Validate the certificate and create A record for the alias",
            Changes: changes,
        },
        HostedZoneId: envHostedZoneID,
    }).promise();
    console.log(`change info for ${option.DomainName}: ${ChangeInfo}`)
    // TODO: handle "PriorRequestNotComplete" error: https://docs.aws.amazon.com/Route53/latest/APIReference/API_ChangeResourceRecordSets.html#API_ChangeResourceRecordSets_Errors

    await envRoute53.waitFor('resourceRecordSetsChanged', {
        // Wait up to 5 minutes
        $waiter: {
            delay: DELAY_RECORD_SETS_CHANGE,
            maxAttempts: ATTEMPTS_RECORD_SETS_CHANGE,
        },
        Id: ChangeInfo.Id,
    }).promise();
}


exports.withSleep = function (s) {
    sleep = s;
};
exports.reset = function () {
    sleep = defaultSleep;

};
exports.withDeadlineExpired = function (d) {
    exports.deadlineExpired = d;
};