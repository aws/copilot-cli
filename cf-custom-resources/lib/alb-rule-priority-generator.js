// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const aws = require("aws-sdk");

// priorityForRootRule is the max priority number that's always set for the listener rule that matches the root path "/"
const priorityForRootRule = "50000";

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
          reject(new Error(`Error ${res.statusCode}: ${res.statusMessage}`));
        } else {
          resolve();
        }
      })
      .end(responseBody, "utf8");
  });
};

/**
 * Lists all the existing rules for a ALB Listener, finds the max of their
 * priorities, and then returns max + 1.
 *
 * @param {string} listenerArn the ARN of the ALB listener.

 * @returns {number} The next available ALB listener rule priority.
 */
const calculateNextRulePriority = async function (listenerArn) {
  var elb = new aws.ELBv2();
  // Grab all the rules for this listener
  var marker;
  var rules = [];
  do {
    const rulesResponse = await elb
      .describeRules({
        ListenerArn: listenerArn,
        Marker: marker,
      })
      .promise();

    rules = rules.concat(rulesResponse.Rules);
    marker = rulesResponse.NextMarker;
  } while (marker);

  let nextRulePriority = 1;
  if (rules.length > 0) {
    // Take the max rule priority, and add 1 to it.
    const rulePriorities = rules.map((rule) => {
      if (
        rule.Priority === "default" ||
        rule.Priority === priorityForRootRule
      ) {
        // Ignore the root rule's priority since it has to be always the max value.
        // Ignore the default rule's prority since it's the same as 0.
        return 0;
      }
      return parseInt(rule.Priority);
    });
    const max = Math.max(...rulePriorities);
    nextRulePriority = max + 1;
  }

  return nextRulePriority;
};

/**
 * Next Available ALB Rule Priority handler, invoked by Lambda
 */
exports.nextAvailableRulePriorityHandler = async function (event, context) {
  var responseData = {};
  const physicalResourceId =
    event.PhysicalResourceId || `alb-rule-priority-${event.LogicalResourceId}`;
  var rulePriority;

  try {
    switch (event.RequestType) {
      case "Create":
        rulePriority = await calculateNextRulePriority(
          event.ResourceProperties.ListenerArn
        );
        responseData.Priority = rulePriority;
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
