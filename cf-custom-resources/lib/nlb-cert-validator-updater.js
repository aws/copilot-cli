// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

const AWS = require('aws-sdk');
const CRYPTO = require("crypto");
const ATTEMPTS_VALIDATION_OPTIONS_READY = 10;
const ATTEMPTS_RECORD_SETS_CHANGE = 10;
const DELAY_RECORD_SETS_CHANGE_IN_S = 30;
const ATTEMPTS_CERTIFICATE_VALIDATED = 19;
const DELAY_CERTIFICATE_VALIDATED_IN_S = 30;

let envHostedZoneID, appName, envName, serviceName, certificateDomain, domainTypes, rootDNSRole, domainName;
let defaultSleep = function (ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
};
let sleep = defaultSleep;
let random = Math.random;

const appRoute53Context = () => {
    let client;
    return () => {
        if (!client) {
            client = new AWS.Route53({
                credentials: new AWS.ChainableTemporaryCredentials({
                    params: { RoleArn: rootDNSRole, },
                    masterCredentials: new AWS.EnvironmentCredentials("AWS"),
                }),
            });
        }
        return client;
    };
}

const envRoute53Context = () => {
    let client;
    return () => {
        if (!client) {
            client = new AWS.Route53();
        }
        return client;
    };
}

const acmContext = () => {
    let client;
    return () => {
        if (!client) {
            client = new AWS.ACM();
        }
        return client;
    };
}

const resourceGroupsTaggingAPIContext = () => {
    let client;
    return () => {
        if (!client) {
            client = new AWS.ResourceGroupsTaggingAPI();
        }
        return client;
    };
}

const clients = {
    app: {
        route53: appRoute53Context(),
    },
    root: {
        route53: appRoute53Context(),
    },
    env: {
        route53:envRoute53Context(),
    },
    acm: acmContext(),
    resourceGroupsTaggingAPI: resourceGroupsTaggingAPIContext(),
}

const appHostedZoneIDContext = () => {
    let id;
    return async () => {
        if (!id) {
            id = await hostedZoneIDByName(`${appName}.${domainName}`);
        }
        return id
    };
}

const rootHostedZoneIDContext = () => {
    let id;
    return async () => {
        if (!id) {
            id = await hostedZoneIDByName(`${domainName}`);
        }
        return id
    };
}

let hostedZoneID = {
    app: appHostedZoneIDContext(),
    root: rootHostedZoneIDContext(),
}

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
    // Destruct resource properties into local variables.
    const props = event.ResourceProperties;
    let {LoadBalancerDNS: loadBalancerDNS,
        LoadBalancerHostedZoneID: loadBalancerHostedZoneID,
    } = props;
    const aliases = new Set(props.Aliases);

    // Initialize global variables.
    envHostedZoneID = props.EnvHostedZoneId;
    envName = props.EnvName;
    appName = props.AppName;
    serviceName = props.ServiceName;
    domainName = props.DomainName;
    rootDNSRole = props.RootDNSRole;
    certificateDomain = `${serviceName}-nlb.${envName}.${appName}.${domainName}`;
    domainTypes = {
        EnvDomainZone: {
            regex: new RegExp(`^([^\.]+\.)?${envName}.${appName}.${domainName}`),
            domain: `${envName}.${appName}.${domainName}`,
        },
        AppDomainZone: {
            regex: new RegExp(`^([^\.]+\.)?${appName}.${domainName}`),
            domain: `${appName}.${domainName}`,
        },
        RootDomainZone: {
            regex: new RegExp(`^([^\.]+\.)?${domainName}`),
            domain: `${domainName}`,
        },
    };

    let aliasesSorted = [...aliases].sort().join(",");
    let physicalResourceID = event.PhysicalResourceId; // The certificate ARN. By default, keep old physical resource ID unchanged.
    let handler = async function() {
        switch (event.RequestType) {
            case "Update":
                let oldAliases = new Set(event.OldResourceProperties.Aliases);
                let oldAliasesSorted = [...oldAliases].sort().join(",");
                if (oldAliasesSorted === aliasesSorted) {
                    break;
                }
                // Fallthrough to "Create". When the aliases are different, the same actions are taken for both "Update" and "Create".
            case "Create":
                await validateAliases(aliases, loadBalancerDNS);
                const certificateARN = await requestCertificate({
                    aliases: aliases,
                    idempotencyToken: CRYPTO
                        .createHash("md5")
                        .update(`/${serviceName}/${aliasesSorted}`)
                        .digest("hex")});
                physicalResourceID = certificateARN; // Update the physical resource ID if a new certificate is created.
                const options = await waitForValidationOptionsToBeReady(certificateARN, aliases);
                await activate(options, certificateARN, loadBalancerDNS, loadBalancerHostedZoneID);
                break;
            case "Delete":
                if (!physicalResourceID || !physicalResourceID.startsWith("arn:")) {
                    // This means no certificate has been created, nor any records. Exit without doing anything.
                    break;
                }
                let unusedOptions = await unusedValidationOptions(physicalResourceID, loadBalancerDNS);
                await clients.acm().deleteCertificate({ CertificateArn: physicalResourceID }).promise().catch(err => {
                    if (err.name !== "ResourceNotFoundException") {
                        throw err;
                    }
                });
                await deactivate(unusedOptions, loadBalancerDNS, loadBalancerHostedZoneID);
                break;
            default:
                throw new Error(`Unsupported request type ${event.RequestType}`);
        }
    };

    try {
        await Promise.race([exports.deadlineExpired(), handler(),]);
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
        let {hostedZoneID, route53Client } = await domainResources(alias);
        const promise = route53Client.listResourceRecordSets({
            HostedZoneId: hostedZoneID,
            MaxItems: "1",
            StartRecordName: alias,
        }).promise().then(({ ResourceRecordSets: recordSet }) => {
            if (!targetRecordExists(alias, recordSet)) {
                return;
            }
            let aliasTarget = recordSet[0].AliasTarget;
            if (aliasTarget && aliasTarget.DNSName.toLowerCase() === `${loadBalancerDNS.toLowerCase()}.`) {
                return; // The record is an alias record and is in use by myself, hence valid.
            }
            if (aliasTarget) {
                throw new Error(`Alias ${alias} is already in use by ${aliasTarget.DNSName}. This could be another load balancer of a different service.`);
            }
            throw new Error(`Alias ${alias} is already in use`);
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
async function requestCertificate({ aliases, idempotencyToken }) {
    const { CertificateArn } = await clients.acm().requestCertificate({
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
            }
        ],
        ValidationMethod: "DNS"
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
        const { Certificate } = await clients.acm().describeCertificate({
            CertificateArn: certificateARN,
        }).promise();
        const options = Certificate.DomainValidationOptions || [];
        options.forEach(option => {
            if (option.ResourceRecord && (aliases.has(option.DomainName) || option.DomainName.toLowerCase() === certificateDomain.toLowerCase())) {
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
    await clients.acm().waitFor("certificateValidated", {
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

    let {hostedZoneID, route53Client} = await domainResources(option.DomainName);
    let { ChangeInfo } = await route53Client.changeResourceRecordSets({
        ChangeBatch: {
            Comment: "Validate the certificate and create A record for the alias",
            Changes: changes,
        },
        HostedZoneId: hostedZoneID,
    }).promise();

    await route53Client.waitFor('resourceRecordSetsChanged', {
        // Wait up to 5 minutes
        $waiter: {
            delay: DELAY_RECORD_SETS_CHANGE_IN_S,
            maxAttempts: ATTEMPTS_RECORD_SETS_CHANGE,
        },
        Id: ChangeInfo.Id,
    }).promise();
}

/**
 * Retrieve validation options that will be unused by any service.
 *
 * @param {String} ownedCertARN The ARN of the certificate that this custom resource manages.
 * @param {String} loadBalancerDNS The DNS of the load balancer used by this service.
 * @returns {Promise<Set<Object>>}
 */
async function unusedValidationOptions(ownedCertARN, loadBalancerDNS) {
    // Look for validation options that will be no longer needed by this service.
    const certificates = await serviceCertificates();
    const { certOwned: certPendingDeletion, otherCerts: certInUse } = categorizeCertificates(certificates, ownedCertARN);
    if (!certPendingDeletion) {
        // Cannot find the certificate that is pending deletion; perhaps it is deleted already. Exit peacefully.
        return new Set();
    }
    let optionsPendingDeletion = await unusedOptionsByService(certPendingDeletion, certInUse);

    // For each of the options pending deletion, validate if it is in use by other services. If it is, Copilot
    // will not delete it.
    let promises = [];
    for (const option of optionsPendingDeletion) {
        const domainName = option["DomainName"];
        // NOTE: The client is initialized outside of the `inUseByOtherServices` function because AWS-SDK mocks cannot
        // mock its API calls if it is initialized in a callback.
        let route53Client;
        try {
            ({route53Client} = await domainResources(domainName));
        } catch (err) {
            // NOTE: The UnrecognizedDomainTypeError is swallowed here because it is preferably handled inside
            // `inUseByOtherServices`.
            if (!err instanceof UnrecognizedDomainTypeError) {
                throw err;
            }
        }
        const promise = inUseByOtherServices(loadBalancerDNS, domainName, route53Client).then((isUsed) => {
            if (isUsed) {
                optionsPendingDeletion.delete(option);
            }
        });
        promises.push(promise);
    }
    await Promise.all(promises);
    return optionsPendingDeletion;
}

async function deactivate(unusedOptions, loadBalancerDNS, loadBalancerHostedZoneID) {
    let promises = [];
    for (let option of unusedOptions) {
        promises.push(deactivateOption(option, loadBalancerDNS, loadBalancerHostedZoneID));
    }
    await Promise.all(promises);
}

async function deactivateOption(option, loadBalancerDNS, loadBalancerHostedZoneID) {
    let changes = [{
        Action: "DELETE",
        ResourceRecordSet: {
            Name: option.ResourceRecord.Name,
            Type: option.ResourceRecord.Type,
            TTL: 60,
            ResourceRecords: [
                {
                    Value: option.ResourceRecord.Value,
                }
            ],
        }
    }];

    if (option.Domain !== certificateDomain) {
        changes.push({
            Action: "DELETE", // It is validated that if the alias is in use, it is in use by the service itself.
            ResourceRecordSet: {
                Name: option.DomainName,
                Type: "A",
                AliasTarget: {
                    DNSName: loadBalancerDNS,
                    EvaluateTargetHealth: true,
                    HostedZoneId: loadBalancerHostedZoneID,
                }
            }
        });
    }

    let {hostedZoneID, route53Client} = await domainResources(option.DomainName);
    let changeResourceRecordSetsInput = {
        ChangeBatch: {
            Comment: `Delete the validation record and A record for ${option.DomainName}`,
            Changes: changes,
        },
        HostedZoneId: hostedZoneID,
    }
    let changeInfo;
    try {
        ({ ChangeInfo: changeInfo } = await route53Client.changeResourceRecordSets(changeResourceRecordSetsInput).promise());
    } catch (e) {
        let recordSetNotFoundErrMessageRegex = /Tried to delete resource record set \[name='.*', type='A'] but it was not found/;
        if (e.message.search(recordSetNotFoundErrMessageRegex) !== -1) {
            return; // If we attempt to `DELETE` a record that doesn't exist, the job is already done, skip waiting.
        }
        throw new Error(`delete record ${option.ResourceRecord.Name}: ` + e.message);
    }

    await route53Client.waitFor('resourceRecordSetsChanged', {
        // Wait up to 5 minutes
        $waiter: {
            delay: DELAY_RECORD_SETS_CHANGE_IN_S,
            maxAttempts: ATTEMPTS_RECORD_SETS_CHANGE,
        },
        Id: changeInfo.Id,
    }).promise();
}

/**
 * Retrieve all certificates used for the service and cache the results.
 * @returns {Array<Object>} An array of descriptions for the certificates used by the service.
 */
async function serviceCertificates() {
    let { ResourceTagMappingList } = await clients.resourceGroupsTaggingAPI().getResources({
        TagFilters: [
            {
                Key: "copilot-application",
                Values: [appName],
            },
            {
                Key: "copilot-environment",
                Values: [envName],
            },
            {
                Key: "copilot-service",
                Values: [serviceName],
            }
        ],
        ResourceTypeFilters: ["acm:certificate"]
    }).promise();

    let certificates = [];
    let promises = [];
    for (const {ResourceARN: arn} of ResourceTagMappingList) {
        let promise = clients.acm().describeCertificate({
            CertificateArn: arn
        }).promise().then(( { Certificate } ) => {
            certificates.push(Certificate);
        });
        promises.push(promise);
    }
    await Promise.all(promises);
    return certificates;
}

/**
 * Retrieve the validation options that are pending deletion. An option is pending deletion if it is only used to
 * validate a certificate that is pending deletion.
 * @param {Object} certPendingDeletion The certificate that is pending deletion.
 * @param {Array<Object>} certsInUse
 * @returns {Promise<Set<Object>>} options that are pending deletion.
 */
async function unusedOptionsByService(certPendingDeletion, certsInUse) {
    let optionsPendingDeletion = new Map();
    for (const option of certPendingDeletion["DomainValidationOptions"]) {
        if (option["ResourceRecord"]) {
            optionsPendingDeletion.set(JSON.stringify(option["ResourceRecord"]), option);
        }
    }
    for (const { DomainValidationOptions: validationOptions } of certsInUse) {
        for (const option of validationOptions) {
            if (option["ResourceRecord"]) {
                optionsPendingDeletion.delete(JSON.stringify(option["ResourceRecord"]));
            }
        }
    }
    let options = new Set();
    for (const opt of optionsPendingDeletion.values()) {
        options.add(opt);
    }
    return options;
}

/**
 * Validate if the domain name is currently in use by other services.
 * @param loadBalancerDNS The DNS of the Network Load Balancer used by this service. The domain name in considered in use
 * by this service, not other services, if it is an alias target pointing to this service's load balancer DNS.
 * @param domainName 
 * @param route53Client The Route53 client to use for the domain name. This client can be a app-level client, or an
 * env-level client, depending on the pattern of the domain name. It is initialized outside of the function because
 * AWS-SDK mocks cannot mock the API call if the client is initialized in a callback.
 * @returns {Promise<boolean>} True if it is considered in use; otherwise false.
 */
async function inUseByOtherServices(loadBalancerDNS, domainName, route53Client) {
    let hostedZoneID;
    try {
        ({hostedZoneID} = await domainResources(domainName));
    } catch (err) {
        if (err instanceof UnrecognizedDomainTypeError) {
            console.log(`Found ${domainName} in subject alternative names. 
It does not match any of these patterns: ".<env>.<app>.<domain>"， ".<app>.<domain>" or ".<domain>". 
This is unexpected. We don't error out as it may not cause any issue.`);
            return true; // This option has unrecognized pattern, we can't check if it is in use, so we assume it is in use.
        }
        throw err;
    }
    const { ResourceRecordSets: recordSet } = await route53Client.listResourceRecordSets({
        HostedZoneId: hostedZoneID,
        MaxItems: "1",
        StartRecordName: domainName,
    }).promise();
    if (!targetRecordExists(domainName, recordSet)) {
        return false; // If there is no record using this domain, it is not in use.
    }
    const inUseByMySelf = recordSet[0].AliasTarget && recordSet[0].AliasTarget.DNSName.toLowerCase() === `${loadBalancerDNS.toLowerCase()}.`
    return !inUseByMySelf
}

/**
 * Categorize a list of certificates into the certificate that corresponds to this particular custom resource, and other certificates.
 * @param {Array<Object>} certificates
 * @param {String} ownedCertARN The ARN of the certificate that this custom resource manages.
 * @returns {Object},Array{Object}
 */
function categorizeCertificates(certificates, ownedCertARN) {
    let certOwned;
    let otherCerts = [];
    for (const cert of certificates) {
        if (cert["CertificateArn"].toLowerCase() === ownedCertARN.toLowerCase()) {
            certOwned = cert;
        } else {
            otherCerts.push(cert);
        }
    }
    return { certOwned, otherCerts };
}

/**
 * Validate if the exact record exits in the set of records.
 * @param targetDomainName The domain name that the target record should have
 * @param recordSet
 * @returns {boolean}
 */
function targetRecordExists(targetDomainName, recordSet) {
    if (!recordSet || recordSet.length === 0) {
        return false;
    }
    return recordSet[0].Name === `${targetDomainName}.`;
}

async function hostedZoneIDByName(domain) {
    const { HostedZones } = await clients.app.route53()
        .listHostedZonesByName({
            DNSName: domain,
            MaxItems: "1",
        }).promise();
    if (!HostedZones || HostedZones.length === 0) {
        throw new Error( `Couldn't find any Hosted Zone with DNS name ${domainName}.`);
    }
    return HostedZones[0].Id.split("/").pop();
}

async function domainResources (alias) {
    if (domainTypes.EnvDomainZone.regex.test(alias)) {
        return {
            domain: domainTypes.EnvDomainZone.domain,
            route53Client: clients.env.route53(),
            hostedZoneID: envHostedZoneID,
        };
    }
    if (domainTypes.AppDomainZone.regex.test(alias)) {
        return {
            domain: domainTypes.AppDomainZone.domain,
            route53Client: clients.app.route53(),
            hostedZoneID: await hostedZoneID.app(),
        };
    }
    if (domainTypes.RootDomainZone.regex.test(alias)) {
        return {
            domain: domainTypes.RootDomainZone.domain,
            route53Client: clients.root.route53(),
            hostedZoneID: await hostedZoneID.root(),
        };
    }
    throw new UnrecognizedDomainTypeError(`unrecognized domain type for ${alias}`);
}

function setEqual(setA, setB) {
    if (setA.size !== setB.size) {
        return false;
    }

    for (let elem of setA) {
        if (!setB.has(elem)) {
            return false;
        }
    }
    return true;
}

function UnrecognizedDomainTypeError(message = "") {
    this.message = message;
}
UnrecognizedDomainTypeError.prototype = Object.create(Error.prototype, {
    constructor: {
        value: Error,
        enumerable: false,
        writable: true,
        configurable: true
    }
});


exports.deadlineExpired = function () {
    return new Promise(function (resolve, reject) {
        setTimeout(
            reject,
            14 * 60 * 1000 + 30 * 1000 /* 14.5 minutes*/,
            new Error(`Lambda took longer than 14.5 minutes to update custom domain`)
        );
    });
};

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
