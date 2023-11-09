// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const aws = require("aws-sdk");

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
 * Delete all objects in a bucket.
 *
 * @param {string} bucketName Name of the bucket to be cleaned.
 */
const cleanBucket = async function (bucketName) {
  const s3 = new aws.S3();
  // Make sure the bucket exists.
  try {
    await s3.headBucket({ Bucket: bucketName }).promise();
  } catch (err) {
    if (err.name === "ResourceNotFoundException") {
      return;
    }
    throw err;
  }
  const listObjectVersionsParam = {
    Bucket: bucketName
  }
  while (true) {
    const listResp = await s3.listObjectVersions(listObjectVersionsParam).promise();
    // After deleting other versions, remove delete markers version.
    // For info on "delete marker": https://docs.aws.amazon.com/AmazonS3/latest/dev/DeleteMarker.html
    let objectsToDelete = [
      ...listResp.Versions.map(version => ({ Key: version.Key, VersionId: version.VersionId })),
      ...listResp.DeleteMarkers.map(marker => ({ Key: marker.Key, VersionId: marker.VersionId }))
    ];
    if (objectsToDelete.length === 0) {
      return
    }
    const delResp = await s3.deleteObjects({
      Bucket: bucketName,
      Delete: {
        Objects: objectsToDelete,
        Quiet: true
      }
    }).promise()
    if (delResp.Errors.length > 0) {
      throw new AggregateError([new Error(`${delResp.Errors.length}/${objectsToDelete.length} objects failed to delete`),
      new Error(`first failed on key "${delResp.Errors[0].Key}": ${delResp.Errors[0].Message}`)]);
    }
    if (!listResp.IsTruncated) {
      return
    }
    listObjectVersionsParam.KeyMarker = listResp.NextKeyMarker
    listObjectVersionsParam.VersionIdMarker = listResp.NextVersionIdMarker
  }
};

/**
 * Correct desired count handler, invoked by Lambda.
 */
exports.handler = async function (event, context) {
  var responseData = {};
  const props = event.ResourceProperties;
  const physicalResourceId = event.PhysicalResourceId || `bucket-cleaner-${event.LogicalResourceId}`;

  try {
    switch (event.RequestType) {
      case "Create":
      case "Update":
        break;
      case "Delete":
        await cleanBucket(props.BucketName);
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
      `${err.message} (Log: ${defaultLogGroup || context.logGroupName}/${defaultLogStream || context.logStreamName
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

class AggregateError extends Error {
  #errors;
  name = "AggregateError";
  constructor(errors) {
    let message = errors
      .map(error =>
        String(error),
      )
      .join("\n");
    super(message);
    this.#errors = errors;
  }
  get errors() {
    return [...this.#errors];
  }
}