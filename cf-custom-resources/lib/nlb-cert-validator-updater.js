// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

const AWS = require('aws-sdk');
const CRYPTO = require("crypto");
const ATTEMPTS_VALIDATION_OPTIONS_READY = 10;
const ATTEMPTS_RECORD_SETS_CHANGE = 10;
const DELAY_RECORD_SETS_CHANGE_IN_S = 30;
const ATTEMPTS_CERTIFICATE_VALIDATED = 19;
const DELAY_CERTIFICATE_VALIDATED_IN_S = 30;

let envHostedZoneID;
let appName, envName, serviceName, certificateDomain, domainTypes, rootDNSRole, domainName;
let defaultSleep = function (ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
};
let sleep = defaultSleep;
let random = Math.random;

let cachedServiceCertificates;

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
    cachedServiceCertificates = null; // Avoid using cached certificates from the last invocation.

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
                const certificateARN = await requestCertificate({
                    aliases: aliases,
                    idempotencyToken: CRYPTO
                        .createHash("md5")
                        .update(physicalResourceID)
                        .digest("hex")});
                const options = await waitForValidationOptionsToBeReady(certificateARN, aliases);
                await activate(options, certificateARN, loadBalancerDNS, loadBalancerHostedZoneID);
                break;
            case "Update":
            case "Delete":
                await unusedValidationOptions(aliases, loadBalancerDNS);
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
            if (aliasTarget && aliasTarget.DNSName === `${loadBalancerDNS}.`) {
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
        const { Certificate } = await clients.acm().describeCertificate({
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
 * Retrieve all certificates used for the service and cache the results.
 * @returns {Array<Object>} An array of descriptions for the certificates used by the service.
 */
async function serviceCertificates() {
    if (cachedServiceCertificates) {
        return cachedServiceCertificates;
    }
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

    cachedServiceCertificates = [];
    let promises = [];
    for (let {ResourceARN: arn} of ResourceTagMappingList) {
        let promise = clients.acm().describeCertificate({
            CertificateArn: arn
        }).promise().then(( { Certificate} ) => {
            cachedServiceCertificates.push(Certificate);
        });
        promises.push(promise);
    }
    await Promise.all(promises);
    return cachedServiceCertificates;
}

/**
 * Retrieve validation options that will be unused by the service after deleting the certificate.
 * An option is deemed pending unused if it validates only the certificate(s) pending deletion.
 *
 * @param {Set<String>} aliases is the subject alternative names of the certificate(s) pending deletion.
 * @returns {Set<Object>} The options that will be unused by the service.
 */
async function optionsUnusedByService(aliases) {
    // NOTE: `Set` uses object reference to identify elements. That is, two options with the same value will not be
    // identify as the same option. Therefore, we use map with object string as keys to identify object with actual
    // values instead of object reference.
    let optionsInUse = new Map();
    let optionsPendingUnusedByService = new Map();

    let certificates = await serviceCertificates();
    for (const {DomainName: domainName, SubjectAlternativeNames: sans, DomainValidationOptions: validationOptions} of certificates) {
        if (domainName === certificateDomain && setEqual(aliases.add(certificateDomain), new Set(sans))) {
            // This is a certificate pending deletion. It is:
            // 1. Copilot-tagged, 2. use the default certificate domain name, and 3. uses the SANs that need to be deleted.
            for (let option of validationOptions) {
                optionsPendingUnusedByService.set(JSON.stringify(option), option);
            }
            continue;
        }
        // Otherwise, this is a certificate that we do not want to touch; validation options of this certificate are deemed in use.
        for (let option of validationOptions) {
            optionsInUse.set(JSON.stringify(option), option);
        }
    }
    for (const optionInUseKey of optionsInUse.keys()) {
        optionsPendingUnusedByService.delete(optionInUseKey);
    }
    let options = new Set();
    for (const optionPendingUnused of optionsPendingUnusedByService.values()) {
        options.add(optionPendingUnused);
    }
    return options;
}

/**
 * Retrieve validation options that will be unused by any service.
 *
 * @param {Set<String>} aliases
 * @param {String} loadBalancerDNS
 * @returns {Promise<Set<any>>}
 */
async function unusedValidationOptions(aliases, loadBalancerDNS) {
    let optionsPendingUnused = await optionsUnusedByService(aliases);
    let promises = [];
    for (let option of optionsPendingUnused) {
        let domainName = option["DomainName"];
        let hostedZoneID, route53Client;
        try {
            ({hostedZoneID, route53Client} = await domainResources(domainName));
        } catch (err) {
            if (!err instanceof UnrecognizedDomainTypeError) {
                throw err;
            }
            // This option has unrecognized pattern, we can't check if it is in use, so we skip deleting it.
            optionsPendingUnused.delete(option);
            continue;
        }
        const promise = route53Client.listResourceRecordSets({
            HostedZoneId: hostedZoneID,
            MaxItems: "1",
            StartRecordName: domainName,
        }).promise().then(({ ResourceRecordSets: recordSet }) => {
            if (!targetRecordExists(domainName, recordSet)) {
                return; // If there is no record using this domain, it is not in use.
            }
            let aliasTarget = recordSet[0].AliasTarget;
            if (aliasTarget && aliasTarget.DNSName !== `${loadBalancerDNS}.`) {
                return; // If it is an A record in use by myself, it is pending unused.
            }
            optionsPendingUnused.delete(option); // Otherwise, it is in use by other services.
        });
        promises.push(promise);
    }
    await Promise.all(promises);
    return optionsPendingUnused;
}

function targetRecordExists(targetDomainName, recordSet) {
    if (!recordSet || recordSet.length === 0) {
        return false;
    }
    return recordSet[0].Name === targetDomainName;
}

exports.deadlineExpired = function () {
    return new Promise(function (resolve, reject) {
        setTimeout(
            reject,
            14 * 60 * 1000 + 30 * 1000 /* 14.5 minutes*/,
            new Error(`Lambda took longer than 14.5 minutes to update custom domain`)
        );
    });
};

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

/**
 * Returns true of setA is equal to setB.
 * @param setA
 * @param setB
 * @returns {boolean}
 */
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
UnrecognizedDomainTypeError.prototype = Error.prototype;

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
