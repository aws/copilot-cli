// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/* jshint node: true */
/* jshint esversion: 8 */

"use strict";

const aws = require("aws-sdk");

const defaultSleep = function (ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
};

// These are used for test purposes only
let defaultResponseURL;
let defaultLogGroup;
let defaultLogStream;
let waiter;
let sleep = defaultSleep;
let random = Math.random;
let maxAttempts = 10;
let domainTypes;

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
let report = function (
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

    let responseBody = JSON.stringify({
      Status: responseStatus,
      Reason: reason,
      PhysicalResourceId: physicalResourceId || context.logStreamName,
      StackId: event.StackId,
      RequestId: event.RequestId,
      LogicalResourceId: event.LogicalResourceId,
      Data: responseData,
    });

    const parsedUrl = new URL(event.ResponseURL || defaultResponseURL);
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
};

const aliasesChanged = function (
    oldResourceProperties,
    aliases,
) {
  if (!oldResourceProperties) {
    return true;
  }
  let prevAliases = getAllAliases(
      oldResourceProperties.Aliases
  );
  let aliasesToDelete = [...prevAliases,].filter(function (itm) {
    return !aliases.has(itm);
  });
  let aliasesToAdd = [...aliases,].filter(function (itm) {
    return !prevAliases.has(itm);
  });
  return aliasesToAdd.length + aliasesToDelete.length;
};

/**
 * Requests a public certificate from AWS Certificate Manager, using DNS validation
 * (see https://docs.aws.amazon.com/acm/latest/userguide/dns-validation.html).
 *
 * @param {string} requestId the CloudFormation request ID
 * @param {string} appName the application name
 * @param {string} envName the environment name
 * @param {string} certDomain the domain of the certificate
 * @param {string[]} aliases the custom domain aliases
 * @param {string[]} sansToUse the subject alternative name to add to the certificate
 * @param {object} acm the AWS ACM client to use for sending requests
 * @returns {string} ARN of the requested certificate
 */
const requestCertificate = async function (
  requestId,
  appName,
  envName,
  certDomain,
  aliases,
  sansToUse,
  acm
) {
  const crypto = require("crypto");
  return acm
    .requestCertificate({
      DomainName: certDomain,
      SubjectAlternativeNames: sansToUse,
      IdempotencyToken: crypto
        .createHash("sha256")
        .update(requestId)
        .digest("hex")
        .substr(0, 32),
      ValidationMethod: "DNS",
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
    })
    .promise();
};

/**
 * Wait until the validation options are ready
 *
 * @param certificateARN the requested certificate for which we are waiting for the validation options to be ready
 * @param sansToUse the subject alternative names added to the certificate
 * @param acm the client to use
 * @returns the validation options
 */
const waitForValidationOptionsToBeReady = async function(
    certificateARN,
    sansToUse,
    acm,
){
  let options;
  let attempt;
  const expectedValidationOptionsNum = sansToUse.length;
  for (attempt = 0; attempt < maxAttempts; attempt++) {
    const { Certificate } = await acm
        .describeCertificate({
          CertificateArn: certificateARN,
        })
        .promise();
    options = Certificate.DomainValidationOptions || [];
    let readyRecordsNum = 0;
    for (const option of options) {
      if (option.ResourceRecord) {
        readyRecordsNum++;
      }
    }
    if (readyRecordsNum === expectedValidationOptionsNum) {
      break;
    }
    // Exponential backoff with jitter based on 200ms base
    // component of backoff fixed to ensure minimum total wait time on
    // slow targets.
    const base = Math.pow(2, attempt);
    await sleep(random() * base * 50 + base * 150);
  }
  if (attempt === maxAttempts) {
    throw new Error(
        `DescribeCertificate did not contain DomainValidationOptions after ${maxAttempts} tries.`
    );
  }
  return options;
};

/**
 * Validate DNS in the root, app, and env hosted zones in parallel, and wait until the certificate is validated.
 * The root hosted zone is created when the user purchases example.com in route53 in their app account.
 * We create the app hosted zone "app.example.com" when running "app init" part of the application stack.
 * The env hosted zone "env.app.example.com" is created when running "env init" part of the env stack.
 *
 * @param options
 * @param envRoute53
 * @param appRoute53
 * @param envHostedZoneId
 * @param certificateARN
 * @param acm
 * @returns {Promise<>} promise for whether the wait succeeds
 */
const validateCertificate = async function(
    options,
    envRoute53,
    appRoute53,
    envHostedZoneId,
    certificateARN,
    acm
){
  await updateHostedZoneRecords(
      "UPSERT",
      options,
      envRoute53,
      appRoute53,
      envHostedZoneId
  );

  await acm
    .waitFor("certificateValidated", {
      // Wait up to 9 minutes and 30 seconds
      $waiter: {
        delay: 30,
        maxAttempts: 19,
      },
      CertificateArn: certificateARN,
    })
    .promise();
};

const updateHostedZoneRecords = async function (
  action,
  options,
  envRoute53,
  appRoute53,
  envHostedZoneId
) {
  const promises = [];
  for (const option of options) {
    const domainType = await getDomainType(option.DomainName);
    switch (domainType) {
      case domainTypes.EnvDomainZone:
        promises.push(
          validateDomain({
            route53: envRoute53,
            record: option.ResourceRecord,
            action: action,
            domainName: "",
            hostedZoneId: envHostedZoneId,
          })
        );
        break;
      case domainTypes.AppDomainZone:
        promises.push(
          validateDomain({
            route53: appRoute53,
            record: option.ResourceRecord,
            action: action,
            domainName: domainType.domain,
          })
        );
        break;
      case domainTypes.RootDomainZone:
        promises.push(
          validateDomain({
            route53: appRoute53,
            record: option.ResourceRecord,
            action: action,
            domainName: domainType.domain,
          })
        );
        break;
    }
  }
  return Promise.all(promises);
};

// deleteHostedZoneRecords deletes the validation records associated with the certificate.
// We don't want to delete a validation record if it's used by another certificate because
// the validation records are used to renew the certificate.
const deleteHostedZoneRecords = async function (
  oldCertOptions,
  oldCertArn,
  defaultDomain,
  envRoute53,
  appRoute53,
  acm,
  envHostedZoneId
) {
  let listCertificatesInput = {};
  let newCertOptions = [];
  // Look for the new certificate, and note down its validation records.
  // This is to make sure if there is a new certificate, we don't delete any DNS validation records that are used by the new certificate.
  let isNewCertFound = false;
  while (!isNewCertFound) {
    const listCertResp = await acm
      .listCertificates(listCertificatesInput)
      .promise();
    for (const certSummary of listCertResp.CertificateSummaryList || []) {
      if (
        certSummary.DomainName !== defaultDomain ||
        certSummary.CertificateArn === oldCertArn
      ) {
        // Skip if it is not the new certificate for the domain.
        continue;
      }
      // There exists another certificate created by Copilot which has the updated alias fields as SANs.
      // We don't want to delete any validation records associated with the new certificate.
      const { Certificate } = await acm
        .describeCertificate({
          CertificateArn: certSummary.CertificateArn,
        })
        .promise();
      newCertOptions = Certificate.DomainValidationOptions || [];
      isNewCertFound = true;
      break;
    }
    if (!listCertResp.NextToken) {
      break;
    }
    listCertificatesInput.NextToken = listCertResp.NextToken;
  }
  const newCertSANs = new Set(newCertOptions.map((item) => item.DomainName));
  const recordOptionsToDelete = [];
  for (const oldCertOption of oldCertOptions) {
    if (!newCertSANs.has(oldCertOption.DomainName)) {
      // This alias field is no longer in use, we can safely delete its validation.
      recordOptionsToDelete.push(oldCertOption);
    }
  }
  // Make sure DNS validation records are unique. For example: "example.com" and "*.example.com"
  // might have the same DNS validation record.
  const filteredRecordOption = [];
  let uniqueValidateRecordNames = new Set();
  for (const option of recordOptionsToDelete) {
    let id = `${option.ResourceRecord.Name} ${option.ResourceRecord.Value}`;
    if (uniqueValidateRecordNames.has(id)) {
      continue;
    }
    uniqueValidateRecordNames.add(id);
    filteredRecordOption.push(option);
  }
  try {
    await updateHostedZoneRecords(
        "DELETE",
        filteredRecordOption,
        envRoute53,
        appRoute53,
        envHostedZoneId
    );
  } catch (e) {
    let recordSetNotFoundErrMessageRegex = new RegExp(".*Tried to delete resource record set.*but it was not found.*");
    if (recordSetNotFoundErrMessageRegex.test(e.message)) {
      return; // If we attempt to `DELETE` a record that doesn't exist, the job is already done, skip waiting.
    }
    throw new Error(`delete record ${option.ResourceRecord.Name}: ` + e.message);
  }
};

const validateDomain = async function ({
  route53,
  record,
  action,
  domainName,
  hostedZoneId,
}) {
  if (!hostedZoneId) {
    const hostedZones = await route53
      .listHostedZonesByName({
        DNSName: domainName,
        MaxItems: "1",
      })
      .promise();
    if (!hostedZones.HostedZones || hostedZones.HostedZones.length === 0) {
      throw new Error(
        `Couldn't find any Hosted Zone with DNS name ${domainName}.`
      );
    }
    hostedZoneId = hostedZones.HostedZones[0].Id.split("/").pop();
  }
  console.log(
    `${action} DNS record into Hosted Zone ${hostedZoneId}: ${record.Name} ${record.Type} ${record.Value}`
  );
  const changeBatch = await updateRecords(
    route53,
    hostedZoneId,
    action,
    record.Name,
    record.Type,
    record.Value
  );
  await waitForRecordChange(route53, changeBatch.ChangeInfo.Id);
};

/**
 * Deletes a certificate from AWS Certificate Manager (ACM) by its ARN.
 * Specifically, if it is the last certificate attaching to the listener, it will also remove the CNAME records
 * for validation in all the root, app, and env hosted zones in parallel.
 * If the certificate does not exist, the function will return normally.
 *
 * @param {string} arn The certificate ARN
 * @param {string} certDomain the domain of the certificate
 * @param {string} envHostedZoneId the environment Route53 Hosted Zone ID
 * @param {string} rootDnsRole the IAM role ARN that can manage domainName
 * @param {string} region the environment region
 */
const deleteCertificate = async function (
  arn,
  certDomain,
  region,
  envHostedZoneId,
  rootDnsRole
) {
  const [acm, envRoute53, appRoute53] = clients(region, rootDnsRole);
  try {
    console.log(`Waiting for certificate ${arn} to become unused`);

    let inUseByResources;
    let options;

    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      const { Certificate } = await acm
        .describeCertificate({
          CertificateArn: arn,
        })
        .promise();

      inUseByResources = Certificate.InUseBy || [];
      options = Certificate.DomainValidationOptions || [];
      let ok = false;
      for (const option of options) {
        if (!option.ResourceRecord) {
          ok = false;
          break;
        }
        ok = true;
      }
      if (!ok || inUseByResources.length) {
        // Deleting resources can be quite slow - so just sleep 30 seconds between checks.
        await sleep(30000);
      } else {
        break;
      }
    }
    if (inUseByResources.length) {
      throw new Error(
        `Certificate still in use after checking for ${maxAttempts} attempts.`
      );
    }

    await deleteHostedZoneRecords(
      options,
      arn,
      certDomain,
      envRoute53,
      appRoute53,
      acm,
      envHostedZoneId
    );

    await acm
      .deleteCertificate({
        CertificateArn: arn,
      })
      .promise();
  } catch (err) {
    if (err.name !== "ResourceNotFoundException") {
      throw err;
    }
  }
};

const waitForRecordChange = function (route53, changeId) {
  return route53
    .waitFor("resourceRecordSetsChanged", {
      // Wait up to 5 minutes
      $waiter: {
        delay: 30,
        maxAttempts: 10,
      },
      Id: changeId,
    })
    .promise();
};

const updateRecords = function (
  route53,
  hostedZone,
  action,
  recordName,
  recordType,
  recordValue
) {
  return route53
    .changeResourceRecordSets({
      ChangeBatch: {
        Changes: [
          {
            Action: action,
            ResourceRecordSet: {
              Name: recordName,
              Type: recordType,
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
      HostedZoneId: hostedZone,
    })
    .promise();
};

// getAllAliases gets all aliases out from a string. For example:
// {"frontend": ["test.foobar.com", "foobar.com"], "api": ["api.foobar.com"]} will return
// ["test.foobar.com", "foobar.com", "api.foobar.com"].
const getAllAliases = function (aliases) {
  let obj;
  try {
    obj = JSON.parse(aliases || "{}");
  } catch (error) {
    throw new Error(`Cannot parse ${aliases} into JSON format.`);
  }
  let aliasList = [];

  for (const [_, aliases] of Object.entries(obj)) {
    aliasList.push(...aliases);
  }

  return new Set(
    aliasList.filter(function (itm) {
      return getDomainType(itm) !== domainTypes.OtherDomainZone;
    })
  );
};

const getDomainType = function (alias) {
  if (domainTypes.EnvDomainZone.regex.test(alias)) {
    return domainTypes.EnvDomainZone;
  }
  if (domainTypes.AppDomainZone.regex.test(alias)) {
    return domainTypes.AppDomainZone;
  }
  if (domainTypes.RootDomainZone.regex.test(alias)) {
    return domainTypes.RootDomainZone;
  }
  return domainTypes.OtherDomainZone;
};

const clients = function (region, rootDnsRole) {
  const acm = new aws.ACM({
    region,
  });
  const envRoute53 = new aws.Route53();
  const appRoute53 = new aws.Route53({
    credentials: new aws.ChainableTemporaryCredentials({
      params: { RoleArn: rootDnsRole, },
      masterCredentials: new aws.EnvironmentCredentials("AWS"),
    }),
  });
  if (waiter) {
    // Used by the test suite, since waiters aren't mockable yet
    envRoute53.waitFor = appRoute53.waitFor = acm.waitFor = waiter;
  }
  return [acm, envRoute53, appRoute53, ];
};

/**
 * Main certificate manager handler, invoked by Lambda
 */
exports.certificateRequestHandler = async function (event, context) {
  let responseData = {};
  let physicalResourceId = event.PhysicalResourceId;
  const props = event.ResourceProperties;
  const [app, env, domain] = [props.AppName, props.EnvName, props.DomainName, ];
  domainTypes = {
    EnvDomainZone: {
      regex: new RegExp(`^([^\.]+\.)?${env}.${app}.${domain}`),
      domain: `${env}.${app}.${domain}`,
    },
    AppDomainZone: {
      regex: new RegExp(`^([^\.]+\.)?${app}.${domain}`),
      domain: `${app}.${domain}`,
    },
    RootDomainZone: {
      regex: new RegExp(`^([^\.]+\.)?${domain}`),
      domain: `${domain}`,
    },
    OtherDomainZone: {},
  };

  let certDomain = `${props.EnvName}.${props.AppName}.${props.DomainName}`;
  let aliases = getAllAliases(props.Aliases);
  const uniqueSansToUse = new Set([certDomain, `*.${certDomain}`, ]);
  for (const alias of aliases) {
    uniqueSansToUse.add(alias);
  }
  const sansToUse = [...uniqueSansToUse, ];
  const [acm, envRoute53, appRoute53] = clients(props.Region, props.RootDNSRole);

  try {
    let response = {};
    let options = {};
    switch (event.RequestType) {
      case "Create":
        response = await requestCertificate(
          event.RequestId,
          props.AppName,
          props.EnvName,
          certDomain,
          aliases,
          sansToUse,
          acm
        );
        responseData.Arn = physicalResourceId = response.CertificateArn;  // Set physicalResourceId as soon as we can.
        options = await waitForValidationOptionsToBeReady(response.CertificateArn, sansToUse, acm);
        await validateCertificate(options, envRoute53, appRoute53, props.EnvHostedZoneId, response.CertificateArn, acm);
        break;
      case "Update":
        // Exit early if cert doesn't change.
        if (!aliasesChanged(event.OldResourceProperties, aliases)) {
          break;
        }
        response = await requestCertificate(
          event.RequestId,
          props.AppName,
          props.EnvName,
          certDomain,
          aliases,
          sansToUse,
          acm
        );
        responseData.Arn = physicalResourceId = response.CertificateArn;
        options = await waitForValidationOptionsToBeReady(response.CertificateArn, sansToUse, acm);
        await validateCertificate(options, envRoute53, appRoute53, props.EnvHostedZoneId, response.CertificateArn, acm);
        break;
      case "Delete":
        // If the resource didn't create correctly, the physical resource ID won't be the
        // certificate ARN, so don't try to delete it in that case.
        if (physicalResourceId.startsWith("arn:")) {
          await deleteCertificate(
            physicalResourceId,
            certDomain,
            props.Region,
            props.EnvHostedZoneId,
            props.RootDNSRole
          );
        }
        break;
      default:
        throw new Error(`Unsupported request type ${event.RequestType}`);
    }
    await report(event, context, "SUCCESS", physicalResourceId, responseData);
  } catch (err) {
    console.log(`Caught error ${err}.`);
    await report(
      event,
      context,
      "FAILED",
      physicalResourceId,
      null,
      `${err.message} (Log: ${defaultLogGroup || context.logGroupName}/${
        defaultLogStream || context.logStreamName
      })`
    );
  }
};

/**
 * @private
 */
exports.withDefaultResponseURL = function (url) {
  defaultResponseURL = url;
};

/**
 * @private
 */
exports.withWaiter = function (w) {
  waiter = w;
};

/**
 * @private
 */
exports.withSleep = function (s) {
  sleep = s;
};

/**
 * @private
 */
exports.reset = function () {
  sleep = defaultSleep;
  random = Math.random;
  waiter = undefined;
  maxAttempts = 10;
};

/**
 * @private
 */
exports.withRandom = function (r) {
  random = r;
};

/**
 * @private
 */
exports.withMaxAttempts = function (ma) {
  maxAttempts = ma;
};

/**
 * @private
 */
exports.withDefaultLogStream = function (logStream) {
  defaultLogStream = logStream;
};

/**
 * @private
 */
exports.withDefaultLogGroup = function (logGroup) {
  defaultLogGroup = logGroup;
};
