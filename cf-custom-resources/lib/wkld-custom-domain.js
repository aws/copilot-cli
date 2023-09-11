// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

const AWS = require("aws-sdk");
const ATTEMPTS_VALIDATION_OPTIONS_READY = 10;
const ATTEMPTS_RECORD_SETS_CHANGE = 10;
const DELAY_RECORD_SETS_CHANGE_IN_S = 30;

let envHostedZoneID, appName, envName, serviceName, domainTypes, rootDNSRole, domainName;
let defaultSleep = function (ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
};
let sleep = defaultSleep;

const appRoute53Context = () => {
  let client;
  return () => {
    if (!client) {
      client = new AWS.Route53({
        credentials: new AWS.ChainableTemporaryCredentials({
          params: { RoleArn: rootDNSRole },
          masterCredentials: new AWS.EnvironmentCredentials("AWS"),
        }),
      });
    }
    return client;
  };
};

const envRoute53Context = () => {
  let client;
  return () => {
    if (!client) {
      client = new AWS.Route53();
    }
    return client;
  };
};

const acmContext = () => {
  let client;
  return () => {
    if (!client) {
      client = new AWS.ACM();
    }
    return client;
  };
};

const resourceGroupsTaggingAPIContext = () => {
  let client;
  return () => {
    if (!client) {
      client = new AWS.ResourceGroupsTaggingAPI();
    }
    return client;
  };
};

const clients = {
  app: {
    route53: appRoute53Context(),
  },
  root: {
    route53: appRoute53Context(),
  },
  env: {
    route53: envRoute53Context(),
  },
  acm: acmContext(),
  resourceGroupsTaggingAPI: resourceGroupsTaggingAPIContext(),
};

const appHostedZoneIDContext = () => {
  let id;
  return async () => {
    if (!id) {
      id = await hostedZoneIDByName(`${appName}.${domainName}`);
    }
    return id;
  };
};

const rootHostedZoneIDContext = () => {
  let id;
  return async () => {
    if (!id) {
      id = await hostedZoneIDByName(`${domainName}`);
    }
    return id;
  };
};

let hostedZoneID = {
  app: appHostedZoneIDContext(),
  root: rootHostedZoneIDContext(),
};

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
function report(event, context, responseStatus, physicalResourceId, responseData, reason) {
  return new Promise((resolve, reject) => {
    const https = require("https");
    const { URL } = require("url");

    let reasonWithLogInfo = `${reason} (Log: ${context.logGroupName}/${context.logStreamName})`;
    let responseBody = JSON.stringify({
      Status: responseStatus,
      Reason: reasonWithLogInfo,
      PhysicalResourceId: physicalResourceId,
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
  let { PublicAccessDNS: publicAccessDNS, PublicAccessHostedZoneID: publicAccessHostedZoneID } = props;
  const aliases = new Set(props.Aliases);

  // Initialize global variables.
  envHostedZoneID = props.EnvHostedZoneId;
  envName = props.EnvName;
  appName = props.AppName;
  serviceName = props.ServiceName;
  domainName = props.DomainName;
  rootDNSRole = props.RootDNSRole;
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

  // The PhysicalResourceID never changes because LogicalResourceId never changes.
  // Therefore, a "Replacement" should never happen.
  const physicalResourceID = event.LogicalResourceId;
  let handler = async function () {
    switch (event.RequestType) {
      case "Update":
        // Hosted Zone and DNS are not guaranteed to be the same, 
        // so we want to be able to update routing in case alias is unchanged but hosted zone or DNS is not.
        let oldAliases = new Set(event.OldResourceProperties.Aliases);
        let oldHostedZoneId = event.OldResourceProperties.PublicAccessHostedZoneID;
        let oldDNS = event.OldResourceProperties.PublicAccessDNS;
        if (setEqual(oldAliases, aliases) && oldHostedZoneId === publicAccessHostedZoneID && oldDNS === publicAccessDNS) {
          break;
        }
        await validateAliases(aliases, publicAccessDNS, oldDNS);
        await activate(aliases, publicAccessDNS, publicAccessHostedZoneID);
        let unusedAliases = new Set([...oldAliases].filter((a) => !aliases.has(a)));
        await deactivate(unusedAliases, oldDNS, oldHostedZoneId);
        break;
      case "Create":
        await validateAliases(aliases, publicAccessDNS);
        await activate(aliases, publicAccessDNS, publicAccessHostedZoneID);
        break;
      case "Delete":
        await deactivate(aliases, publicAccessDNS, publicAccessHostedZoneID);
        break;
      default:
        throw new Error(`Unsupported request type ${event.RequestType}`);
    }
  };

  try {
    await Promise.race([exports.deadlineExpired(), handler()]);
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
 * @param {String} publicAccessDNS the DNS of the service's load balancer.
 * @param {String} oldPublicAccessDNS the old DNS of the service's load balancer.
 * @throws error if at least one of the aliases is not valid.
 */
async function validateAliases(aliases, publicAccessDNS, oldPublicAccessDNS) {
  let promises = [];

  for (let alias of aliases) {
    let { hostedZoneID, route53Client } = await domainResources(alias);
    const promise = route53Client
      .listResourceRecordSets({
        HostedZoneId: hostedZoneID,
        MaxItems: "1",
        StartRecordName: alias,
        StartRecordType: "A",
      })
      .promise()
      .then(({ ResourceRecordSets: recordSet }) => {
        if (!targetRecordExists(alias, recordSet)) {
          return;
        }
        if (recordSet[0].Type !== "A") {
          return;
        }
        let aliasTarget = recordSet[0].AliasTarget;
        if (aliasTarget && aliasTarget.DNSName.toLowerCase() === `${publicAccessDNS.toLowerCase()}.`) {
          return; // The record is an alias record and is in use by myself, hence valid.
        }
        if (aliasTarget && oldPublicAccessDNS && aliasTarget.DNSName.toLowerCase() === `${oldPublicAccessDNS.toLowerCase()}.`) {
          return; // The record was used by the old DNS, therefore is now used by the current DNS, hence valid.
        }
        if (aliasTarget) {
          throw new Error(`Alias ${alias} is already in use by ${aliasTarget.DNSName}. This could be another load balancer of a different service.`);
        }
        throw new Error(`Alias ${alias} is already in use`);
      });
    promises.push(promise);
  }
  await Promise.all(promises);
}

/**
 * Add A-records for the aliases
 * @param {Set<String>}aliases
 * @param {String} publicAccessDNS
 * @param {String} publicAccessHostedZone
 * @returns {Promise<void>}
 */
async function activate(aliases, publicAccessDNS, publicAccessHostedZone) {
  let promises = [];
  for (let alias of aliases) {
    promises.push(activateAlias(alias, publicAccessDNS, publicAccessHostedZone));
  }
  await Promise.all(promises);
}

/**
 * Add an A-record that points to the load balancer DNS as an alias target for the alias to its corresponding hosted zone.
 * @param {String} alias
 * @param {String} publicAccessDNS
 * @param {String} publicAccessHostedZone
 * @returns {Promise<void>}
 */
async function activateAlias(alias, publicAccessDNS, publicAccessHostedZone) {
  // NOTE: It has been validated that if the alias is in use, it is in use by the service itself.
  // Therefore, an "UPSERT" will not overwrite a record that belongs to another service.
  let changes = [
    {
      Action: "UPSERT",
      ResourceRecordSet: {
        Name: alias,
        Type: "A",
        AliasTarget: {
          DNSName: publicAccessDNS,
          EvaluateTargetHealth: true,
          HostedZoneId: publicAccessHostedZone,
        },
      },
    },
  ];

  let { hostedZoneID, route53Client } = await domainResources(alias);
  let { ChangeInfo } = await route53Client
    .changeResourceRecordSets({
      ChangeBatch: {
        Comment: `Upsert A-record for alias ${alias}`,
        Changes: changes,
      },
      HostedZoneId: hostedZoneID,
    })
    .promise();

  await route53Client
    .waitFor("resourceRecordSetsChanged", {
      // Wait up to 5 minutes
      $waiter: {
        delay: DELAY_RECORD_SETS_CHANGE_IN_S,
        maxAttempts: ATTEMPTS_RECORD_SETS_CHANGE,
      },
      Id: ChangeInfo.Id,
    })
    .promise();
}

/**
 *
 * @param {Set<String>} aliases
 * @param {String} publicAccessDNS
 * @param {String} publicAccessHostedZoneID
 * @returns {Promise<void>}
 */
async function deactivate(aliases, publicAccessDNS, publicAccessHostedZoneID) {
  let promises = [];
  for (let alias of aliases) {
    promises.push(deactivateAlias(alias, publicAccessDNS, publicAccessHostedZoneID));
  }
  await Promise.all(promises);
}

/**
 * Remove the A-record of an alias that points to the load balancer DNS from its corresponding hosted zone.
 *
 * @param {String} alias
 * @param {String} publicAccessDNS
 * @param {String} publicAccessHostedZoneID
 * @returns {Promise<void>}
 */
async function deactivateAlias(alias, publicAccessDNS, publicAccessHostedZoneID) {
  // NOTE: It has been validated that if the alias is in use, it is in use by the service itself.
  // Therefore, an "UPSERT" will not overwrite a record that belongs to another service.
  let changes = [
    {
      Action: "DELETE",
      ResourceRecordSet: {
        Name: alias,
        Type: "A",
        AliasTarget: {
          DNSName: publicAccessDNS,
          EvaluateTargetHealth: true,
          HostedZoneId: publicAccessHostedZoneID,
        },
      },
    },
  ];

  let { hostedZoneID, route53Client } = await domainResources(alias);
  let changeResourceRecordSetsInput = {
    ChangeBatch: {
      Comment: `Delete the A-record for ${alias}`,
      Changes: changes,
    },
    HostedZoneId: hostedZoneID,
  };
  let changeInfo;
  try {
    ({ ChangeInfo: changeInfo } = await route53Client.changeResourceRecordSets(changeResourceRecordSetsInput).promise());
  } catch (e) {
    let recordSetNotFoundErrMessageRegex = new RegExp(".*Tried to delete resource record set.*but it was not found.*");
    if (recordSetNotFoundErrMessageRegex.test(e.message)) {
      return; // If we attempt to `DELETE` a record that doesn't exist, the job is already done, skip waiting.
    }

    let recordSetNotMatchedErrMessageRegex = new RegExp(".*Tried to delete resource record set.*but the values provided do not match the current values.*");
    if (recordSetNotMatchedErrMessageRegex.test(e.message)) {
      // NOTE: The alias target, or record value is not exactly what we provided
      // E.g. the alias target DNS name is another load balancer or cloudfront distribution
      // This service should not delete the A-record if it is not being pointed to.
      // However, this is an unexpected situation, we should log this information.
      console.log(`Received error when trying to delete A-record for ${alias}: ${e.message}. Perhaps the alias record isn't pointing to ${publicAccessDNS}.`);
      return;
    }
    throw new Error(`delete record ${alias}: ` + e.message);
  }

  await route53Client
    .waitFor("resourceRecordSetsChanged", {
      // Wait up to 5 minutes
      $waiter: {
        delay: DELAY_RECORD_SETS_CHANGE_IN_S,
        maxAttempts: ATTEMPTS_RECORD_SETS_CHANGE,
      },
      Id: changeInfo.Id,
    })
    .promise();
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
  const { HostedZones } = await clients.app
    .route53()
    .listHostedZonesByName({
      DNSName: domain,
      MaxItems: "1",
    })
    .promise();
  if (!HostedZones || HostedZones.length === 0) {
    throw new Error(`Couldn't find any Hosted Zone with DNS name ${domainName}.`);
  }
  return HostedZones[0].Id.split("/").pop();
}

async function domainResources(alias) {
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
    configurable: true,
  },
});

exports.deadlineExpired = function () {
  return new Promise(function (resolve, reject) {
    setTimeout(reject, 14 * 60 * 1000 + 30 * 1000 /* 14.5 minutes*/, new Error(`Lambda took longer than 14.5 minutes to update custom domain`));
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
