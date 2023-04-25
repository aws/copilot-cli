// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

describe("copy assets", () => {
  const aws = require("aws-sdk-mock");
  const lambdaTester = require("lambda-tester").noVersionCheck();
  const sinon = require("sinon");
  const handler = require("../lib/copy-assets");

  afterEach(() => {
    aws.restore();
  });

  test("happy path", () => {
    const fake = sinon.fake.resolves({});
    aws.mock("S3", "copyObject", fake);

    return lambdaTester(handler.handler)
      .event({
        srcBucket: "mockSrcBucket",
        destBucket: "mockDestBucket",
        mapping: {
          path: "mockPath",
          destPath: "mockDestPath",
        }
      })
      .expectResolve(result => {
        sinon.assert.calledWith(fake, {
          CopySource: "mockSrcBucket/mockPath",
          Bucket: "mockDestBucket",
          Key: "mockDestPath"
        });
      });
  });

  test("s3 error", () => {
    const fake = sinon.fake.rejects("some error");
    aws.mock("S3", "copyObject", fake);

    return lambdaTester(handler.handler)
      .event({
        srcBucket: "mockSrcBucket",
        destBucket: "mockDestBucket",
        mapping: {
          path: "mockPath",
          destPath: "mockDestPath",
        }
      })
      .expectReject(err => {
        sinon.assert.calledWith(fake, {
          CopySource: "mockSrcBucket/mockPath",
          Bucket: "mockDestBucket",
          Key: "mockDestPath"
        });
        expect(err).toEqual(new Error("some error"));
      });
  });
});