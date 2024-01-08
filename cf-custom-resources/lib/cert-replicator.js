// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/* jshint node: true */
/* jshint esversion: 8 */

"use strict";

const { ACM, waitUntilCertificateValidated, DescribeCertificateCommand, RequestCertificateCommand, DeleteCertificateCommand } = require("@aws-sdk/client-acm");

const defaultSleep = function (ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
};

// These are used for test purposes only
let defaultResponseURL;
let defaultLogGroup;
let defaultLogStream;
let sleep = defaultSleep;
let maxAttempts = 10;

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
      PhysicalResourceId:
        physicalResourceId || defaultLogStream || context.logStreamName,
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
 * Replicate a public certificate from AWS Certificate Manager in the target region.
 *
 * @param {string} requestId the CloudFormation request ID
 * @param {string} appName the application name
 * @param {string} envName the environment name
 * @param {string} certArn arn for the certificate to replicate from
 * @param {string} envRegionAcm acm client in environment region
 * @param {string} targetRegionAcm acm client in target region
 * @returns {string} ARN of the replicated certificate
 */
const replicateCertificate = async function (
  requestId,
  appName,
  envName,
  certArn,
  envRegionAcm,
  targetRegionAcm
) {
  const { Certificate } = await envRegionAcm
    .send(new DescribeCertificateCommand({
      CertificateArn: certArn,
    }));
  const domainName = Certificate.DomainName;
  const sans = Certificate.SubjectAlternativeNames;

  const crypto = require("crypto");
  return await targetRegionAcm
    .send(new RequestCertificateCommand({
      DomainName: domainName,
      SubjectAlternativeNames: sans,
      IdempotencyToken: crypto
        .createHash("sha256")
        .update(requestId)
        .digest("hex")
        .substr(0, 32),
      ValidationMethod: "DNS",
      Tags: [
        {
          Key: "copilot-application",
          Value: appName,
        },
        {
          Key: "copilot-environment",
          Value: envName,
        },
      ],
    })
    );
};

/**
 * Deletes a certificate from AWS Certificate Manager (ACM) by its ARN.
 * If the certificate does not exist, the function will return normally.
 *
 * @param {string} arn The certificate ARN
 * @param {string} targetRegionAcm acm client in target region
 */
const deleteCertificate = async function (arn, acm) {
  try {
    let inUseByResources = [];

    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      const { Certificate } = await acm
        .send(new DescribeCertificateCommand({
          CertificateArn: arn,
        }));

      inUseByResources = Certificate.InUseBy || [];
      if (inUseByResources.length === 0) {
        break;
      }
      // Deleting resources can be quite slow - so just sleep 30 seconds between checks.
      await sleep(30000);
    }
    if (inUseByResources.length) {
      throw new Error(
        `Certificate still in use by ${inUseByResources.join()} after checking for ${maxAttempts} attempts.`
      );
    }

    await acm
      .send(new DeleteCertificateCommand({
        CertificateArn: arn,
    }));
  } catch (err) {
    if (err.name !== "ResourceNotFoundException") {
      throw err;
    }
  }
};

const validateCertificate = async function (certificateARN, acm) {
  // Wait up to 9 minutes and 30 seconds
  await waitUntilCertificateValidated({
    client:acm,
    maxWaitTime: 570,
    minDelay: 30,
    maxDelay: 30,
  },{
    CertificateArn: certificateARN
  });
};

/**
 * Main certificate replicator handler, invoked by Lambda
 */
exports.certificateReplicateHandler = async function (event, context) {
  let responseData = {};
  let physicalResourceId = event.PhysicalResourceId;
  const props = event.ResourceProperties;
  const [targetRegion, envRegion, certArn] = [
    props.TargetRegion,
    props.EnvRegion,
    props.CertificateArn,
  ];
  let handler = async function () {
    // Configure clients.
    const envRegionAcm = new ACM({ region: envRegion });
    const targetRegionAcm = new ACM({ region: targetRegion });
    switch (event.RequestType) {
      case "Create":
      case "Update":
        const response = await replicateCertificate(
          event.RequestId,
          props.AppName,
          props.EnvName,
          certArn,
          envRegionAcm,
          targetRegionAcm
        );
        responseData.Arn = physicalResourceId = response.CertificateArn;
        await exports.validateCertificate(response.CertificateArn, targetRegionAcm);
        break;
      case "Delete":
        // If the resource didn't create correctly, the physical resource ID won't be the
        // certificate ARN, so don't try to delete it in that case.
        if (physicalResourceId.startsWith("arn:")) {
          await deleteCertificate(physicalResourceId, targetRegionAcm);
        }
        break;
      default:
        throw new Error(`Unsupported request type ${event.RequestType}`);
    }
  };
  try {
    await Promise.race([exports.deadlineExpired(), handler()]);
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

exports.deadlineExpired = function () {
  return new Promise(function (resolve, reject) {
    setTimeout(
      reject,
      14 * 60 * 1000 + 30 * 1000 /* 14.5 minutes*/,
      new Error("Lambda took longer than 14.5 minutes to replicate certificate")
    );
  });
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
exports.withSleep = function (s) {
  sleep = s;
};

/**
 * @private
 */
exports.reset = function () {
  sleep = defaultSleep;
  maxAttempts = 10;
};

/**
 * @private
 */
exports.withMaxAttempts = function (ma) {
  maxAttempts = ma;
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

/**
 * @private
 */
exports.withDeadlineExpired = function (d) {
  exports.deadlineExpired = d;
};

/**
 * @private
 */
exports.validateCertificate = function(certificateARN, acm) {
  return validateCertificate(certificateARN, acm);
};

