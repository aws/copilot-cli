// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const aws = require("aws-sdk");

const changeRecordAction = {
  Upsert: "UPSERT",
  Delete: "DELETE",
}

// These are used for test purposes only
let defaultResponseURL;
let defaultLogGroup;
let defaultLogStream;
let waiter;

let hostedZoneCache = new Map();

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

    var responseBody = JSON.stringify({
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

/**
 * Upsert all alias records to the correct domain hosted zone. More specifically,
 * we'll add the record to the root hosted zone for aliases in format of `*.${domainName}`;
 * to the app hosted zone for aliases in format of `*.$appName}.${domainName}`;
 * and to the env hosted zone for aliases in format of `*.${envName}.${appName}.${domainName}`.
 * Also for the other aliases not matching any of the condition above, we'll skip since
 * it is corresponding hosted zone is not managable by Copilot.
 *
 * @param {string} aliases the custom domain aliases
 * @param {string} accessDNS DNS of the public access
 * @param {string} accessHostedZone Hosted Zone of the public access
 * @param {string} rootDnsRole the IAM role ARN that can manage domainName
 * @param {string} aliasTypes the alias type
 */
const writeCustomDomainRecord = async function (
  appRoute53,
  envRoute53,
  aliases,
  accessDNS,
  accessHostedZone,
  aliasTypes,
  action
) {
  const actions = [];
  for (const alias of aliases) {
    const aliasType = await getAliasType(aliasTypes, alias);
    switch (aliasType) {
      case aliasTypes.EnvDomainZone:
        actions.push(writeARecord(
          envRoute53,
          alias,
          accessDNS,
          accessHostedZone,
          aliasType.domain,
          action
        ));
        break;
      case aliasTypes.AppDomainZone:
        actions.push(writeARecord(
          appRoute53,
          alias,
          accessDNS,
          accessHostedZone,
          aliasType.domain,
          action
        ));
        break;
      case aliasTypes.RootDomainZone:
        actions.push(writeARecord(
          appRoute53,
          alias,
          accessDNS,
          accessHostedZone,
          aliasType.domain,
          action
        ));
        break;
      // We'll skip if it is the other alias type since it will be in another account's route53.
      default:
    }
  }
  await Promise.all(actions);
};

const writeARecord = async function (
  route53,
  alias,
  accessDNS,
  accessHostedZone,
  domain,
  action
) {
  let hostedZoneId = hostedZoneCache.get(domain);
  if (!hostedZoneId) {
    const hostedZones = await route53
      .listHostedZonesByName({
        DNSName: domain,
        MaxItems: "1",
      })
      .promise();

    if (!hostedZones.HostedZones || hostedZones.HostedZones.length == 0) {
      throw new Error(`Couldn't find any Hosted Zone with DNS name ${domain}.`);
    }
    hostedZoneId = hostedZones.HostedZones[0].Id.split("/").pop();
    hostedZoneCache.set(domain, hostedZoneId);
  }
  console.log(`${action} A record into Hosted Zone ${hostedZoneId}`);
  try {
    const changeBatch = await updateRecords(
      route53,
      hostedZoneId,
      action,
      alias,
      accessDNS,
      accessHostedZone
    );
    await waitForRecordChange(route53, changeBatch.ChangeInfo.Id);
  } catch (err) {
    if (action === changeRecordAction.Delete && isRecordSetNotFoundErr(err)) {
      console.log(`${err.message}; Copilot is ignoring this record.`);
      return;
    }
    throw err;
  }
};

// Example error message: "InvalidChangeBatch: [Tried to delete resource record set [name='a.domain.com.', type='A'] but it was not found]"
const isRecordSetNotFoundErr = (err) => err.message.includes("Tried to delete resource record set") && err.message.includes("but it was not found")

/**
 * Custom domain handler, invoked by Lambda.
 */
exports.handler = async function (event, context) {
  var responseData = {};
  const physicalResourceId = event.LogicalResourceId;
  const props = event.ResourceProperties;
  const [app, env, domain] = [props.AppName, props.EnvName, props.DomainName];
  var aliasTypes = {
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
    OtherDomainZone: { regex: new RegExp(`.*`) },
  };
  const envRoute53 = new aws.Route53();
  const appRoute53 = new aws.Route53({
    credentials: new aws.ChainableTemporaryCredentials({
      params: { RoleArn: props.AppDNSRole },
      masterCredentials: new aws.EnvironmentCredentials("AWS"),
    }),
  });
  if (waiter) {
    // Used by the test suite, since waiters aren't mockable yet
    envRoute53.waitFor = appRoute53.waitFor = waiter;
  }
  try {
    var aliases = await getAllAliases(props.Aliases);
    switch (event.RequestType) {
      case "Create":
        await writeCustomDomainRecord(
          appRoute53,
          envRoute53,
          aliases,
          props.PublicAccessDNS,
          props.PublicAccessHostedZone,
          aliasTypes,
          changeRecordAction.Upsert,
        );
        break;
      case "Update":
        await writeCustomDomainRecord(
          appRoute53,
          envRoute53,
          aliases,
          props.PublicAccessDNS,
          props.PublicAccessHostedZone,
          aliasTypes,
          changeRecordAction.Upsert,
        );
        // After upserting new aliases, delete unused ones. For example: previously we have ["foo.com", "bar.com"],
        // and now the aliases param is updated to just ["foo.com"] then we'll delete "bar.com".
        var prevAliases = await getAllAliases(
          event.OldResourceProperties.Aliases
        );
        var aliasesToDelete = [...prevAliases].filter(function (itm) {
          return !aliases.has(itm);
        });
        await writeCustomDomainRecord(
          appRoute53,
          envRoute53,
          aliasesToDelete,
          props.PublicAccessDNS,
          props.PublicAccessHostedZone,
          aliasTypes,
          changeRecordAction.Delete,
        );
        break;
      case "Delete":
        await writeCustomDomainRecord(
          appRoute53,
          envRoute53,
          aliases,
          props.PublicAccessDNS,
          props.PublicAccessHostedZone,
          aliasTypes,
          changeRecordAction.Delete
        );
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
  var aliasList = [];
  for (var m in obj) {
    aliasList.push(...obj[m]);
  }
  return new Set(aliasList);
};

const getAliasType = function (aliasTypes, alias) {
  switch (true) {
    case aliasTypes.EnvDomainZone.regex.test(alias):
      return aliasTypes.EnvDomainZone;
    case aliasTypes.AppDomainZone.regex.test(alias):
      return aliasTypes.AppDomainZone;
    case aliasTypes.RootDomainZone.regex.test(alias):
      return aliasTypes.RootDomainZone;
    default:
      return aliasTypes.OtherDomainZone;
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
  alias,
  accessDNS,
  accessHostedZone
) {
  return route53
    .changeResourceRecordSets({
      ChangeBatch: {
        Changes: [
          {
            Action: action,
            ResourceRecordSet: {
              Name: alias,
              Type: "A",
              AliasTarget: {
                HostedZoneId: accessHostedZone,
                DNSName: accessDNS,
                EvaluateTargetHealth: true,
              },
            },
          },
        ],
      },
      HostedZoneId: hostedZone,
    })
    .promise();
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
exports.reset = function () {
  waiter = undefined;
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
