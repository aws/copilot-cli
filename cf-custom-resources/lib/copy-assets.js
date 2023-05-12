// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

const aws = require("aws-sdk");

/**
 * Main handler, invoked by Lambda
 */
exports.handler = async function (event, context, callback) {
  const s3 = new aws.S3();
  await s3
    .copyObject({
      CopySource: event.srcBucket + "/" + event.mapping.path,
      Bucket: event.destBucket,
      Key: event.mapping.destPath,
      ContentType:
        event.mapping.contentType !== ""
          ? event.mapping.contentType
          : undefined,
      // Required otherwise ContentType won't be applied.
      // See https://github.com/aws/aws-sdk-js/issues/1092 for more.
      MetadataDirective: "REPLACE",
    })
    .promise();
};
