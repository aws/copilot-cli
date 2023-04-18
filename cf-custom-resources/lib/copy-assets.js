// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

const aws = require("aws-sdk");

/**
 * Main handler, invoked by Lambda
 */
exports.handler = async function (event, context, callback) {
	const handler = async function () {
		const s3 = new aws.S3();
		const params = {
			CopySource: event.srcBucket + "/" + event.mapping.path,
			Bucket: event.destBucket,
			Key: event.mapping.destPath
		};

		await s3.copyObject(params).promise();
	};

	await Promise.race([deadlineExpired(), handler()]);
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