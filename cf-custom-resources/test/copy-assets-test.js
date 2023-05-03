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

  test("copies file if file doesn't exist", () => {
    const err = new Error();
    err.statusCode = 404;
    const headObjectFake = sinon.fake.rejects(err);
    aws.mock("S3", "headObject", headObjectFake);

    const copyObjectFake = sinon.fake.resolves({});
    aws.mock("S3", "copyObject", copyObjectFake);

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
        sinon.assert.calledWith(headObjectFake, {
          Bucket: "mockDestBucket",
          Key: "mockDestPath"
        });

        sinon.assert.calledWith(copyObjectFake, {
          CopySource: "mockSrcBucket/mockPath",
          Bucket: "mockDestBucket",
          Key: "mockDestPath"
        });
      });
  });

  test("skips copy if file already exists", () => {
    const headObjectFake = sinon.fake.resolves({});
    aws.mock("S3", "headObject", headObjectFake);

    const copyObjectFake = sinon.fake.resolves({});
    aws.mock("S3", "copyObject", copyObjectFake);

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
        sinon.assert.calledWith(headObjectFake, {
          Bucket: "mockDestBucket",
          Key: "mockDestPath"
        });

        sinon.assert.notCalled(copyObjectFake);
      });
  });

  test("errors on failure checking if file exists", () => {
    const err = new Error("headObject error");
    err.statusCode = 403;
    const headObjectFake = sinon.fake.rejects(err);
    aws.mock("S3", "headObject", headObjectFake);

    const copyObjectFake = sinon.fake.resolves({});
    aws.mock("S3", "copyObject", copyObjectFake);

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
        sinon.assert.calledWith(headObjectFake, {
          Bucket: "mockDestBucket",
          Key: "mockDestPath"
        });

        sinon.assert.notCalled(copyObjectFake);

        expect(err).toEqual(new Error("headObject error"));
      });
  });

  test("errors on failure to copy object", () => {
    const err = new Error();
    err.statusCode = 404;
    const headObjectFake = sinon.fake.rejects(err);
    aws.mock("S3", "headObject", headObjectFake);

    const copyObjectFake = sinon.fake.rejects("copyObject error");
    aws.mock("S3", "copyObject", copyObjectFake);

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
        sinon.assert.calledWith(headObjectFake, {
          Bucket: "mockDestBucket",
          Key: "mockDestPath"
        });

        sinon.assert.calledWith(copyObjectFake, {
          CopySource: "mockSrcBucket/mockPath",
          Bucket: "mockDestBucket",
          Key: "mockDestPath"
        });

        expect(err).toEqual(new Error("copyObject error"));
      });
  });
});