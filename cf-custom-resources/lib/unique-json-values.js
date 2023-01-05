// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

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

const deadlineExpired = function () {
  return new Promise((resolve, reject) => {
    setTimeout(
      reject,
      9 * 60 * 1000 + 30 * 1000 /* 9.5 minutes */,
      new Error("Lambda took longer than 9.5 minutes")
    );
  });
};

/**
 * Main handler, invoked by Lambda
 *
 * The input event.ResourceProperties.Aliases is a map of service name to
 * it's alises. For example, it might look like this:
 * {
 *  "svc1": ["svc1.com", "example.com"],
 *  "svc2": ["example.com"]
 *  "svc3": ["svc3.com"]
 * }
 *
 * The input event.ResourceProperties.FilterFor is a comma delimated list
 * of keys in event.ResourceProperties.Aliases to filter for.
 * Taking the above map, and event.ResourceProperties.FilterFor = "svc1,svc2"
 * The services considered for uniqueness is:
 * {
 *  "svc1": ["svc1.com", "example.com"],
 *  "svc2": ["example.com"]
 * }
 *
 * The input event.ResourceProperties.AdditionalStrings is a string slice (e.g., ["foobar.com"]).
 *
 * This function returns a sorted list of unique values found in the map and the additional string.
 * For this example, UniqueValues would be:
 * ["example.com", "foobar.com", "svc1.com"]
 */
exports.handler = async function (event, context) {
  const responseData = {};
  const physicalResourceId =
    event.PhysicalResourceId || event.LogicalResourceId;

  const handler = async function () {
    switch (event.RequestType) {
      case "Create":
      case "Update":
        const aliasesForService = JSON.parse(
          event.ResourceProperties.Aliases || "{}"
        );
        const filterFor = new Set(
          event.ResourceProperties.FilterFor.split(",")
        );
        const filteredAliasesForService = Object.fromEntries(
          Object.entries(aliasesForService).filter(([key]) =>
            filterFor.has(key)
          )
        );
        const unique = new Set(Object.values(filteredAliasesForService).flat());
        if (event.ResourceProperties.AdditionalStrings) {
          event.ResourceProperties.AdditionalStrings.forEach((element) => {
            if (element) {
              unique.add(element);
            }
          });
        }
        responseData.UniqueValues = Array.from(unique).sort();
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
