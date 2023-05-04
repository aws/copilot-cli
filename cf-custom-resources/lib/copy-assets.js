// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

const aws = require("aws-sdk");

/**
 * Main handler, invoked by Lambda
 */
exports.handler = async function (event, context, callback) {
	const s3 = new aws.S3();
	await s3.copyObject({
		CopySource: event.srcBucket + "/" + event.mapping.path,
		Bucket: event.destBucket,
		Key: event.mapping.destPath
	}).promise();
};