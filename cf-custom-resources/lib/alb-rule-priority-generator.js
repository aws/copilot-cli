// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const aws = require("aws-sdk");

// minPriorityForRootRule is the min priority number for the root path "/".
const minPriorityForRootRule = 48000;
// maxPriorityForRootRule is the max priority number for the root path "/".
const maxPriorityForRootRule = 50000;

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

/**
 * Lists all the existing rules for an ALB Listener, finds the max of their
 * priorities, and then returns max + 1.
 *
 * @param {string} listenerArn the ARN of the ALB listener.

 * @returns {number} The next available ALB listener rule priority.
 */
const calculateNextRulePriority = async function (listenerArn) {
  let rules = await getListenerRules(listenerArn);
  let nextRulePriority = 1;
  if (rules.length > 0) {
    // Take the max rule priority, and add 1 to it.
    const rulePriorities = rules.map((rule) => {
      if (
        rule.Priority === "default" ||
        rule.Priority >= minPriorityForRootRule
      ) {
        // Ignore the root rule's priority.
        // Ignore the default rule's priority since it's the same as 0.
        return 0;
      }
      return parseInt(rule.Priority);
    });
    nextRulePriority = Math.max(...rulePriorities) + 1;
  }
  return nextRulePriority;
};

/**
 * Lists all the existing rules for an ALB Listener, finds the min of their root rule
 * priorities, and then returns min - 1.
 *
 * @param {string} listenerArn the ARN of the ALB listener.

 * @returns {number} The next available ALB listener rule priority.
 */
const calculateNextRootRulePriority = async function (listenerArn) {
  let rules = await getListenerRules(listenerArn);
  let nextRulePriority = maxPriorityForRootRule;
  if (rules.length > 0) {
    // We'll start from the max rule priority number for root path so that
    // it won't override any other rule with the same host header.
    // Then, take the min rule priority among all the root rule, and decrement it by 1.
    const rulePriorities = rules.map((rule) => {
      if (
        rule.Priority === "default" ||
        rule.Priority < minPriorityForRootRule
      ) {
        // Ignore the root rule's priority.
        // Ignore the non root rule's priority.
        return maxPriorityForRootRule + 1;
      }
      return parseInt(rule.Priority);
    });
    nextRulePriority = Math.min(...rulePriorities) - 1;
  }
  return nextRulePriority;
};

const getListenerRules = async function (listenerArn) {
  let elb = new aws.ELBv2();
  // Grab all the rules for this listener
  let marker;
  let rules = [];
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
  return rules;
};

/**
 * Next Available ALB Rule Priority handler, invoked by Lambda
 */
exports.nextAvailableRulePriorityHandler = async function (event, context) {
  let responseData = {};
  let nextRootRuleNumber, nextNonRootRuleNumber;
  const physicalResourceId = event.PhysicalResourceId || `alb-rule-priority-${event.LogicalResourceId}`;
  try {
    switch (event.RequestType) {
      case "Create":
      case "Update":
          for (let i=0; i < event.ResourceProperties.RulePath.length; i++){
            if (event.ResourceProperties.RulePath[i] === "/") {
              if (nextRootRuleNumber == null){
                nextRootRuleNumber = await calculateNextRootRulePriority(
                    event.ResourceProperties.ListenerArn
                );
              }
              if (i == 0) {
                responseData["Priority"]  = nextRootRuleNumber--;
              } else {
                responseData["Priority"+i]  = nextRootRuleNumber--;
              }
            } else {
              if (nextNonRootRuleNumber == null) {
                nextNonRootRuleNumber = await calculateNextRulePriority(
                    event.ResourceProperties.ListenerArn
                );
              }
              if (i == 0) {
                responseData["Priority"] = nextNonRootRuleNumber++;
              } else {
                responseData["Priority"+i] = nextNonRootRuleNumber++;
              }
            }
          }
        break;
      // Do nothing on delete, since this isn't a "real" resource.
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
