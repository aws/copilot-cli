// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

const AWS = require('aws-sdk');
const ATTEMPTS_VALIDATION_OPTIONS_READY = 10;
const ATTEMPTS_RECORD_SETS_CHANGE = 10;
const DELAY_RECORD_SETS_CHANGE_IN_S = 30;
const ATTEMPTS_CERTIFICATE_VALIDATED = 19;
const DELAY_CERTIFICATE_VALIDATED_IN_S = 30;

let acm, envRoute53, envHostedZoneID, appName, envName, serviceName, certificateDomain;
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
        let responseBody = JSON.stringify({
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

    let {LoadBalancerDNS: loadBalancerDNS,
        LoadBalancerHostedZoneID: loadBalancerHostedZoneID,
        DomainName: domainName,
    } = props;
    const aliases = new Set(props.Aliases);

    acm = new AWS.ACM();
    envRoute53 = new AWS.Route53();
    envHostedZoneID = props.EnvHostedZoneId;
    envName = props.EnvName;
    appName = props.AppName;
    serviceName = props.ServiceName;
    certificateDomain = `${serviceName}-nlb.${envName}.${appName}.${domainName}`;

    // NOTE: If the aliases have changed, then we need to replace the certificate being used, as well as deleting/adding
    // validation records and A records. In general, any change in aliases indicate a "replacement" of the resources
    // managed by the custom resource lambda; on the contrary, the same set of aliases indicate that there is no need to
    // replace or update the certificate, nor the validation records or A records. Hence, we can use this as the physicalResourceID.
    let aliasesSorted = [...aliases].sort().join(",");
    const physicalResourceID = `/${serviceName}/${aliasesSorted}`;

    let handler = async function() {
        switch (event.RequestType) {
            case "Create":
                await validateAliases(aliases, loadBalancerDNS);
                const certificateARN = await requestCertificate({aliases: aliases, idempotencyToken: physicalResourceID});
                const options = await waitForValidationOptionsToBeReady(certificateARN, aliases);
                await activate(options, certificateARN, loadBalancerDNS, loadBalancerHostedZoneID);
                break;
            case "Update":
            case "Delete":
            default:
                throw new Error(`Unsupported request type ${event.RequestType}`);
        }
    };

    try {
        await handler();
        await report(event, context, "SUCCESS", physicalResourceID);
    } catch (err) {
        console.log(`Caught error for service ${serviceName}: ${err.message}`);
        await report(event, context, "FAILED", physicalResourceID, null, err.message);
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
    let promises = [];

    for (let alias of aliases) {
        const promise = envRoute53.listResourceRecordSets({
            HostedZoneId: envHostedZoneID,
            MaxItems: "1",
            StartRecordName: alias,
        }).promise().then((data) => {
            let recordSet = data["ResourceRecordSets"];
            if (!recordSet || recordSet.length === 0) {
                return;
            }
            let aliasTarget = recordSet[0].AliasTarget;
            if (aliasTarget && aliasTarget.DNSName === `${loadBalancerDNS}.`) {
                return; // The record is an alias record and is in use by myself, hence valid.
            }

            if (aliasTarget) {
                throw new Error(`Alias ${alias} is in use by ${aliasTarget.DNSName}. This could be another load balancer of a different service.`);
            }
            throw new Error(`Alias ${alias} is in use`);
        })
        promises.push(promise);
    }
    await Promise.all(promises);
}

/**
 * Requests a public certificate from AWS Certificate Manager, using DNS validation.
 *
 * @param {Object} requestCertificateInput is the input to requestCertificate, containing the alias and idempotencyToken.
 * @return {String} The ARN of the requested certificate.
 */
async function requestCertificate(requestCertificateInput) {
    let { aliases, idempotencyToken } = requestCertificateInput
    const { CertificateArn } = await acm.requestCertificate({
        DomainName: certificateDomain,
        IdempotencyToken: idempotencyToken,
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
            {
                Key: "copilot-service",
                Value: serviceName,
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

    await acm.waitFor("certificateValidated", {
        // Wait up to 9 minutes and 30 seconds
        $waiter: {
            delay: DELAY_CERTIFICATE_VALIDATED_IN_S,
            maxAttempts: ATTEMPTS_CERTIFICATE_VALIDATED,
        },
        CertificateArn: certificateARN,
    }).promise();
}

/**
 * Upsert the validation record for the alias, as well as adding the A record if the alias is not the default certificate domain.
 *
 * @param {Object} option
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

    await envRoute53.waitFor('resourceRecordSetsChanged', {
        // Wait up to 5 minutes
        $waiter: {
            delay: DELAY_RECORD_SETS_CHANGE_IN_S,
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
exports.attemptsValidationOptionsReady = ATTEMPTS_VALIDATION_OPTIONS_READY;