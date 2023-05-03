// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

const aws = require("aws-sdk");

/**
 * Main handler, invoked by Lambda
 */
exports.handler = async function (event, context, callback) {
	const s3 = new aws.S3();

	try {
		await s3.headObject({
			Bucket: event.destBucket,
			Key: event.mapping.destPath
		}).promise();

		// if headObject is successful, the file already exists
		// in destBucket. no need to copy, so we're done!
		return;
	} catch (err) {
		// if the object was not found (404),
		// then we'll copy and create the object.
		// if there was some other error, we'll throw that.
		if (err.statusCode !== 404) {
			throw err;
		}
	}

	await s3.copyObject({
		CopySource: event.srcBucket + "/" + event.mapping.path,
		Bucket: event.destBucket,
		Key: event.mapping.destPath
	}).promise();
};