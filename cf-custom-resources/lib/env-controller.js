// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const aws = require("aws-sdk");

// These are used for test purposes only
let defaultResponseURL;

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
 * Control the optional resources of the environment stack by updating the parameters.
 *
 * @param {string} requestType Type of the request.
 * @param {string} stackName Name of the stack.
 * @param {string} workload Name of the copilot workload.
 *
 * @returns {number} The running task number.
 */
const controlEnv = async function (requestType, stackName, workload) {
  var cfn = new aws.CloudFormation();
  while (true) {
    var describeStackResp = await cfn
      .describeStacks({
        StackName: stackName,
      })
      .promise();
    if (describeStackResp.Stacks.length !== 1) {
      throw new Error(`Cannot find environment stack ${stackName}`);
    }
    var updatedEnvStack = describeStackResp.Stacks[0];
    var params = JSON.parse(JSON.stringify(updatedEnvStack.Parameters));
    for (const param of params) {
      if (param.ParameterKey === "ALBWorkloads") {
        param.ParameterValue = updateALBWorkloads(
          requestType,
          workload,
          param.ParameterValue
        );
      }
    }
    const exportedValues = getExportedValues(updatedEnvStack)
    // Return if there's no parameter changes.
    if (JSON.stringify(updatedEnvStack.Parameters) === JSON.stringify(params)) {
      return exportedValues;
    }
    try {
      await cfn
        .updateStack({
          StackName: stackName,
          Parameters: params,
          UsePreviousTemplate: true,
          RoleARN: exportedValues["CFNExecutionRoleARN"],
          Capabilities: [
            "CAPABILITY_IAM",
            "CAPABILITY_NAMED_IAM",
            "CAPABILITY_AUTO_EXPAND",
          ],
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
        })
        .promise();
      continue;
    }
    // Wait until update complete, then return the updated env stack output.
    await cfn
      .waitFor("stackUpdateComplete", {
        StackName: stackName,
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
        responseData = await controlEnv(
          "Create",
          props.EnvStack,
          props.Workload
        );
        physicalResourceId = `envcontoller/${props.EnvStack}/${props.Workload}`;
        break;
      case "Update":
        responseData = await controlEnv(
          "Update",
          props.EnvStack,
          props.Workload
        );
        physicalResourceId = event.PhysicalResourceId;
        break;
      case "Delete":
        await controlEnv("Delete", props.EnvStack, props.Workload);
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

const getExportedValues = function (stack) {
  const exportedValues = {};
  stack.Outputs.forEach((output) => {
    exportedValues[output.OutputKey] = output.OutputValue;
  });
  return exportedValues
}

const updateALBWorkloads = function (requestType, workload, value) {
  var set = new Set(
    value.split(",").filter(function (el) {
      return el != "";
    })
  );
  switch (requestType) {
    case "Create":
      set.add(workload);
      break;
    case "Update":
      set.add(workload);
      break;
    case "Delete":
      set.delete(workload);
      break;
    default:
      throw new Error(`Unsupported request type ${requestType}`);
  }
  return Array.from(set).join(",");
};

/**
 * @private
 */
exports.withDefaultResponseURL = function (url) {
  defaultResponseURL = url;
};
