// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
'use strict';

const aws = require('aws-sdk');

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
let report = function (event, context, responseStatus, physicalResourceId, responseData, reason) {
    return new Promise((resolve, reject) => {
        const https = require('https');
        const {
            URL
        } = require('url');

        var responseBody = JSON.stringify({
            Status: responseStatus,
            Reason: reason,
            PhysicalResourceId: physicalResourceId || context.logStreamName,
            StackId: event.StackId,
            RequestId: event.RequestId,
            LogicalResourceId: event.LogicalResourceId,
            Data: responseData
        });

        const parsedUrl = new URL(event.ResponseURL || defaultResponseURL);
        const options = {
            hostname: parsedUrl.hostname,
            port: 443,
            path: parsedUrl.pathname + parsedUrl.search,
            method: 'PUT',
            headers: {
                'Content-Type': '',
                'Content-Length': responseBody.length
            }
        };

        https.request(options)
            .on('error', reject)
            .on('response', res => {
                res.resume();
                if (res.statusCode >= 400) {
                    reject(new Error(`Error ${res.statusCode}: ${res.statusMessage}`));
                } else {
                    resolve();
                }
            })
            .end(responseBody, 'utf8');
    });
};

/**
 * Get the current running task number for a specific task definition.
 *
 * @param {string} cluster Name of the ECS cluster.
 * @param {string} family Name of the task definition family.

 * @returns {number} The running task number.
 */
const getRunningTaskCount = async function (cluster, family) {
  var ecs = new aws.ECS();

  var taskNum = 0
  var nextToken;
  do {
    const resp = await ecs.listTasks({
      cluster: cluster,
      family: family,
      nextToken: nextToken
    }).promise();
    taskNum += resp.taskArns.length;
    nextToken = resp.nextToken;
  } while (nextToken)

  return taskNum;
};

/**
 * Correct desired count handler, invoked by Lambda.
 */
exports.desiredCountHandler = async function(event, context) {
  var responseData = {};
  var physicalResourceId;

  try {
    switch (event.RequestType) {
      case 'Create':
        responseData.DesiredCount = event.ResourceProperties.DesiredCount;
        physicalResourceId = event.PhysicalResourceId;
        console.log(`Successfully set desired count to ${responseData.DesiredCount}.`);
        break;
      case 'Update':
        responseData.DesiredCount = await getRunningTaskCount(event.ResourceProperties.Cluster, event.ResourceProperties.Family);
        physicalResourceId = event.PhysicalResourceId;
        console.log(`Successfully update desired count to ${responseData.DesiredCount}.`);
        break;
      case 'Delete':
        physicalResourceId = event.PhysicalResourceId;
        break;
      default:
        throw new Error(`Unsupported request type ${event.RequestType}`);
    }

    await report(event, context, 'SUCCESS', physicalResourceId, responseData);
  } catch (err) {
    // If it fails, just set to be desired count and return.
    responseData.DesiredCount = event.ResourceProperties.DesiredCount;
    console.log(`Caught error ${err}. Set back desired count to ${responseData.DesiredCount}`);
    await report(event, context, 'SUCCESS', physicalResourceId, responseData);
  }
};


/**
 * @private
 */
exports.withDefaultResponseURL = function(url) {
  defaultResponseURL = url;
};
