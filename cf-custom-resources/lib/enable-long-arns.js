// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const aws = require("aws-sdk");

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
 * Enables long ARN formats for the authenticated AWS principal. For example, if the Lambda's role
 * is the CFNExecutionRole, then the CFNExecutionRole will have long ARN enabled.
 *
 * By enabling the long ARN format, ECS services, tasks, and container instances become taggable.
 */
const enableLongArnFormat = async function () {
  const longFormats = [
    "serviceLongArnFormat",
    "taskLongArnFormat",
    "containerInstanceLongArnFormat",
  ];
  const ecs = new aws.ECS();
  for (const fmt of longFormats) {
    try {
      await ecs
        .putAccountSetting({
          name: fmt,
          value: "enabled",
        })
        .promise();
    } catch (err) {
      console.log(`enable ${fmt}: ${err}.`);
      throw err;
    }
  }
};

/**
 * Handler invoked by Lambda.
 */
exports.handler = async function (event, context) {
  var responseData = {};
  const physicalResourceId = event.PhysicalResourceId || event.LogicalResourceId;

  try {
    switch (event.RequestType) {
      case "Create":
        await enableLongArnFormat();
        break;
      // Do nothing on update and delete, since this isn't a "real" resource.
      case "Update":
      case "Delete":
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
