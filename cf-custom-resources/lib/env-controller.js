// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const aws = require("aws-sdk");

// These are used for test purposes only
let defaultResponseURL;

const updateStackWaiter = {
  delay: 30,
  maxAttempts: 29,
};

const AliasParamKey = "Aliases";

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
 * Update the environment stack's parameters by adding or removing {workload} from the provided {parameters}.
 *
 * @param {string} stackName Name of the stack.
 * @param {string} workload Name of the copilot workload.
 * @param {string[]} envControllerParameters List of parameters from the environment stack to update.
 *
 * @returns {parameters} The updated parameters.
 */
const controlEnv = async function (
  stackName,
  workload,
  aliases,
  envControllerParameters
) {
  var cfn = new aws.CloudFormation();
  aliases = aliases || [];
  while (true) {
    var describeStackResp = await cfn
      .describeStacks({
        StackName: stackName,
      })
      .promise();
    if (describeStackResp.Stacks.length !== 1) {
      throw new Error(`Cannot find environment stack ${stackName}`);
    }
    const updatedEnvStack = describeStackResp.Stacks[0];
    const envParams = JSON.parse(JSON.stringify(updatedEnvStack.Parameters));
    const envSet = setOfParameterKeysWithWorkload(envParams, workload);
    const shouldUpdateAliases = needUpdateAliases(envParams, workload, aliases);
    const controllerSet = new Set(envControllerParameters);

    const parametersToRemove = [...envSet].filter(
      (param) => !controllerSet.has(param)
    );
    const parametersToAdd = [...controllerSet].filter(
      (param) => !envSet.has(param)
    );
    const exportedValues = getExportedValues(updatedEnvStack);
    // Return if there are no parameter changes.
    if (
      parametersToRemove.length +
        parametersToAdd.length +
        shouldUpdateAliases ===
      0
    ) {
      return exportedValues;
    }

    for (const envParam of envParams) {
      if (envParam.ParameterKey === AliasParamKey) {
        if (shouldUpdateAliases) {
          envParam.ParameterValue = updateAliases(
            workload,
            aliases || [],
            envParam.ParameterValue
          );
        }
        continue;
      }
      if (parametersToRemove.includes(envParam.ParameterKey)) {
        const values = new Set(
          envParam.ParameterValue.split(",").filter(Boolean)
        ); // Filter out the empty string
        // in the output array to prevent a leading comma in the parameters list.
        values.delete(workload);
        envParam.ParameterValue = [...values].join(",");
      }
      if (parametersToAdd.includes(envParam.ParameterKey)) {
        const values = new Set(
          envParam.ParameterValue.split(",").filter(Boolean)
        );
        values.add(workload);
        envParam.ParameterValue = [...values].join(",");
      }
    }

    try {
      await cfn
        .updateStack({
          StackName: stackName,
          Parameters: envParams,
          UsePreviousTemplate: true,
          RoleARN: exportedValues["CFNExecutionRoleARN"],
          Capabilities: updatedEnvStack.Capabilities,
        })
        .promise();
    } catch (err) {
      if (
        !err.message.match(
          /^Stack.*is in UPDATE_IN_PROGRESS state and can not be updated/
        )
      ) {
        throw err;
      }
      // If the other workload is updating the env stack, wait until update completes.
      await cfn
        .waitFor("stackUpdateComplete", {
          StackName: stackName,
          $waiter: updateStackWaiter,
        })
        .promise();
      continue;
    }
    // Wait until update complete, then return the updated env stack output.
    await cfn
      .waitFor("stackUpdateComplete", {
        StackName: stackName,
        $waiter: updateStackWaiter,
      })
      .promise();
    describeStackResp = await cfn
      .describeStacks({
        StackName: stackName,
      })
      .promise();
    if (describeStackResp.Stacks.length !== 1) {
      throw new Error(`Cannot find environment stack ${stackName}`);
    }
    return getExportedValues(describeStackResp.Stacks[0]);
  }
};

/**
 * Environment controller handler, invoked by Lambda.
 */
exports.handler = async function (event, context) {
  var responseData = {};
  var physicalResourceId;
  const props = event.ResourceProperties;

  try {
    switch (event.RequestType) {
      case "Create":
        responseData = await Promise.race([
          exports.deadlineExpired(),
          controlEnv(
            props.EnvStack,
            props.Workload,
            props.Aliases,
            props.Parameters
          ),
        ]);
        physicalResourceId = `envcontoller/${props.EnvStack}/${props.Workload}`;
        break;
      case "Update":
        responseData = await Promise.race([
          exports.deadlineExpired(),
          controlEnv(
            props.EnvStack,
            props.Workload,
            props.Aliases,
            props.Parameters
          ),
        ]);
        physicalResourceId = event.PhysicalResourceId;
        break;
      case "Delete":
        responseData = await Promise.race([
          exports.deadlineExpired(),
          controlEnv(
            props.EnvStack,
            props.Workload,
            [] // Set to empty to denote that Workload should not be included in any env stack parameter.
          ),
        ]);
        physicalResourceId = event.PhysicalResourceId;
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
      err.message
    );
  }
};

function setOfParameterKeysWithWorkload(cfnParams, workload) {
  const envSet = new Set();
  cfnParams.forEach((param) => {
    if (param.ParameterKey !== AliasParamKey) {
      let values = new Set(param.ParameterValue.split(","));
      if (!values.has(workload)) {
        return;
      }
    }
    envSet.add(param.ParameterKey);
  });
  return envSet;
}

function needUpdateAliases(cfnParams, workload, aliases) {
  let requireUpdate = false;
  cfnParams.forEach((param) => {
    if (param.ParameterKey !== AliasParamKey) {
      return;
    }
    let obj = JSON.parse(param.ParameterValue || "{}");
    if ((obj[workload] || []).toString() !== aliases.toString()) {
      requireUpdate = true;
    }
  });
  return requireUpdate;
}

const updateAliases = function (workload, aliases, cfnAliases) {
  let obj = JSON.parse(cfnAliases || "{}");
  if (aliases.length !== 0) {
    obj[workload] = aliases;
  } else {
    obj[workload] = undefined;
  }
  const updatedAliases = JSON.stringify(obj);
  return updatedAliases === "{}" ? "" : updatedAliases;
};

const getExportedValues = function (stack) {
  const exportedValues = {};
  stack.Outputs.forEach((output) => {
    exportedValues[output.OutputKey] = output.OutputValue;
  });
  return exportedValues;
};

/**
 * Update parameter by adding workload to the parameter values.
 *
 * @param {string} requestType type of the request.
 * @param {string} workload name of the workload.
 * @param {string} paramValue value of the parameter.
 *
 * @returns {string} The updated parameter.
 * @returns {bool} whether the parameter is modified.
 */

exports.deadlineExpired = function () {
  return new Promise(function (resolve, reject) {
    setTimeout(
      reject,
      14 * 60 * 1000 + 30 * 1000 /* 14.5 minutes*/,
      new Error("Lambda took longer than 14.5 minutes to update environment")
    );
  });
};

/**
 * @private
 */
exports.withDefaultResponseURL = function (url) {
  defaultResponseURL = url;
};
