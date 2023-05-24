// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

const aws = require("aws-sdk");

/**
 * Main handler, invoked by Lambda
 */
exports.handler = async function (event, context) {
  const responseData = {};
  const physicalResourceId = event.PhysicalResourceId || event.LogicalResourceId;

  const handler = async function () {
    switch (event.RequestType) {
      case "Create":
      case "Update":
        const sf = new aws.StepFunctions();
        const res = await sf.startSyncExecution({
          stateMachineArn: event.ResourceProperties.StateMachineARN,
        }).promise();

        // Even if the execution starts and does not throw an error it does not mean the execution was successful.
        // See https://docs.aws.amazon.com/step-functions/latest/apireference/API_StartSyncExecution.html#StepFunctions-StartSyncExecution-response-status
        if (res.status !== "SUCCEEDED") {
          throw new Error(`State machine failed: ${res.cause}`);
        }
        break;
      case "Delete":
        // Do nothing on delete, since this isn't a "real" resource.
        break;
      default:
        throw new Error(`Unsupported request type ${event.RequestType}`);
    }
  };

  try {
    await Promise.race([deadlineExpired(), handler()]);
    await report(event, context, "SUCCESS", physicalResourceId, responseData);
  } catch (err) {
    console.error(`caught error: ${err}`);
    await report(
      event,
      context,
      "FAILED",
      physicalResourceId,
      null,
      `${err.message} (Log: ${context.logGroupName}/${context.logStreamName})`
    );
  }
};

let deadlineExpired = () => {
  return new Promise((resolve, reject) => {
    setTimeout(
      reject,
      14 * 60 * 1000 /* 14 minutes */,
      new Error("Lambda took longer than 14 minutes")
    );
  });
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
const report = function (
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
};

/**
 * @private
 * withDeadlineExpired overrides the default deadlineExpired function.
 * Used for testing.
 */
exports.withDeadlineExpired = fn => {
  deadlineExpired = fn;
};