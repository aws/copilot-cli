// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";;
const {
  fromEnv,
  fromTemporaryCredentials
} = require("@aws-sdk/credential-providers");

const {
  Route53,
  waitUntilResourceRecordSetsChanged
} = require("@aws-sdk/client-route-53");

// These are used for test purposes only
let defaultResponseURL;
let defaultLogGroup;
let defaultLogStream;

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
          reject(
            new Error(
              `Server returned error ${res.statusCode}: ${res.statusMessage}`
            )
          );
        } else {
          resolve();
        }
      })
      .end(responseBody, "utf8");
  });
};

/**
 * Creates a NS recordset in the domain's hosted zone using the rootDnsRole for the subDomain.
 * This essentially delegates authority for subdomain to the subdomain's hostedzone.
 *
 * The rootDnsRole has to have access to the hostedzone for domainName.
 *
 * @param {string} requestId the CloudFormation request ID
 * @param {string} domainName the DNS name to add to the subDomain to (ecs-cli.aws).
 * @param {string} subDomain the full subdomain to add to the domain above (test.ecs-cli.aws).
 * @param {string[]} nameServers the subdomain nameservers to add to the domain's hostedzone.
 * @param {string} rootDnsRole the IAM role ARN that can manage domainName
 */
const createSubdomainInRoot = async function (
  requestId,
  domainName,
  subDomain,
  nameServers,
  rootDnsRole,
  hostedZoneId
) {
  const route53 = new Route53({
    credentials: // JS SDK v3 switched credential providers from classes to functions.
    // This is the closest approximation from codemod of what your application needs.
    // Reference: https://www.npmjs.com/package/@aws-sdk/credential-providers
    fromTemporaryCredentials({
      params: { RoleArn: rootDnsRole },
      masterCredentials: // JS SDK v3 switched credential providers from classes to functions.
      // This is the closest approximation from codemod of what your application needs.
      // Reference: https://www.npmjs.com/package/@aws-sdk/credential-providers
      fromEnv("AWS"),
    })
  });
  if (!hostedZoneId) {
  const hostedZones = await route53
    .listHostedZonesByName({
      DNSName: domainName,
    });

  if (!hostedZones.HostedZones || hostedZones.HostedZones.length == 0) {
    throw new Error(
      `Couldn't find any hostedzones with DNS name ${domainName}. Request ${requestId}`
    );
  }

  const domainHostedZone = hostedZones.HostedZones[0];

  // HostedZoneIDs are of the form /hostedzone/1234455, but the actual
  // ID is after the last slash.
  hostedZoneId = domainHostedZone.Id.split("/").pop();
  }
  const changeBatch = await route53
    .changeResourceRecordSets({
      ChangeBatch: {
        Changes: [
          recordChangeAction(
            "UPSERT",
            subDomain,
            "NS",
            nameServers.map((ns) => {
              return {
                Value: ns,
              };
            })
          ),
        ],
      },
      HostedZoneId: hostedZoneId,
    });

  console.log(
    `Created recordset in hostedzone ${hostedZoneId} for ${subDomain}`
  );

  await waitForRecordSetChange(route53, changeBatch.ChangeInfo.Id);
};

/**
 * Deletes the NameServer record sets from a hostedzone using a cross
 * account role. If the subdomain doesn't exist, this fast succeeds.
 *
 * @param {string} requestId the CloudFormation request ID
 * @param {string} domainName the DNS name to add to the subDomain to (ecs-cli.aws).
 * @param {string} subDomain the full subdomain to add to the domain above (test.ecs-cli.aws).
 * @param {string} rootDnsRole the IAM role ARN that can manage domainName
 * @returns {string} the deleted subdomain
 */
const deleteSubdomainInRoot = async function (
  requestId,
  domainName,
  subDomain,
  rootDnsRole,
  hostedZoneId
) {
  const route53 = new Route53({
    credentials: // JS SDK v3 switched credential providers from classes to functions.
    // This is the closest approximation from codemod of what your application needs.
    // Reference: https://www.npmjs.com/package/@aws-sdk/credential-providers
    fromTemporaryCredentials({
      params: { RoleArn: rootDnsRole },
      masterCredentials: // JS SDK v3 switched credential providers from classes to functions.
      // This is the closest approximation from codemod of what your application needs.
      // Reference: https://www.npmjs.com/package/@aws-sdk/credential-providers
      fromEnv("AWS"),
    })
  });
  if (!hostedZoneId) {
  const hostedZones = await route53
    .listHostedZonesByName({
      DNSName: domainName,
    });

  if (!hostedZones.HostedZones || hostedZones.HostedZones.length == 0) {
    throw new Error(
      `Couldn't find any hostedzones with DNS name ${domainName}. Request ${requestId}`
    );
  }

  const domainHostedZone = hostedZones.HostedZones[0];

  // HostedZoneIDs are of the form /hostedzone/1234455, but the actual
  // ID is after the last slash.
  hostedZoneId = domainHostedZone.Id.split("/").pop();
  }
  // Find the recordsets for this subdomain, and then remove it
  // from the hosted zone.
  const recordSets = await route53
    .listResourceRecordSets({
      HostedZoneId: hostedZoneId,
      MaxItems: "1",
      StartRecordName: subDomain,
      StartRecordType: "NS",
    });

  // If the records have already been deleted, return early.
  if (!recordSets.ResourceRecordSets || recordSets.ResourceRecordSets == 0) {
    return subDomain;
  }

  const subDomainRecordSet = recordSets.ResourceRecordSets[0];
  // If the our subdomain doesn't exactly match the recordset,
  // or the type isn't NS, we'll skip deleting it - since it isn't our record.
  if (
    subDomainRecordSet.Name !== `${subDomain}.` ||
    subDomainRecordSet.Type !== "NS"
  ) {
    return subDomain;
  }
  console.log(`Deleting recordset ${subDomainRecordSet.Name}`);

  const changeBatch = await route53
    .changeResourceRecordSets({
      ChangeBatch: {
        Changes: [
          recordChangeAction(
            "DELETE",
            subDomain,
            "NS",
            subDomainRecordSet.ResourceRecords
          ),
        ],
      },
      HostedZoneId: hostedZoneId,
    });

  await waitForRecordSetChange(route53, changeBatch.ChangeInfo.Id);
  return subDomain;
};

const recordChangeAction = function (
  action,
  recordName,
  recordType,
  recordValues
) {
  return {
    Action: action,
    ResourceRecordSet: {
      Name: recordName,
      Type: recordType,
      TTL: 60,
      ResourceRecords: recordValues,
    },
  };
};

const waitForRecordSetChange = function (route53, changeId) {
  return waitUntilResourceRecordSetsChanged({
    client: route53,
    minDelay: 30,
    maxWaitTime: 600
  }, {
    Id: changeId
  });
};

exports.domainDelegationHandler = async function (event, context) {
  var responseData = {};
  const props = event.ResourceProperties;
  const physicalResourceId = props.SubdomainName;
  try {
    switch (event.RequestType) {
      case "Create":
      case "Update":
        await createSubdomainInRoot(
          event.RequestId,
          props.DomainName,
          props.SubdomainName,
          props.NameServers,
          props.RootDNSRole,
          props.RootHostedZoneId
        );
        break;
      case "Delete":
        await deleteSubdomainInRoot(
          event.RequestId,
          props.DomainName,
          props.SubdomainName,
          props.RootDNSRole,
          props.RootHostedZoneId
        );
        break;
      default:
        throw new Error(`Unsupported request type ${event.RequestType}`);
    }

    await report(event, context, "SUCCESS", physicalResourceId, responseData);
  } catch (err) {
    console.log(`Caught error ${err}.`);
    console.log(err);
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
exports.withDefaultLogStream = function (logStream) {
  defaultLogStream = logStream;
};

/**
 * @private
 */
exports.withDefaultLogGroup = function (logGroup) {
  defaultLogGroup = logGroup;
};
