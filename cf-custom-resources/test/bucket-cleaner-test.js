// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

describe("Bucket Cleaner", () => {
  const AWS = require("aws-sdk-mock");
  const LambdaTester = require("lambda-tester").noVersionCheck();
  const sinon = require("sinon");
  const bucketCleanerHandler = require("../lib/bucket-cleaner");
  const nock = require("nock");
  const ResponseURL = "https://cloudwatch-response-mock.example.com/";
  const LogGroup = "/aws/lambda/testLambda";
  const LogStream = "2021/06/28/[$LATEST]9b93a7dca7344adeb19asdgc092dbbfd";

  let origLog = console.log;

  const testRequestId = "f4ef1b10-c39a-44e3-99c0-fbf6h23c3943";
  const testBucketName = "myBucket"

  beforeEach(() => {
    bucketCleanerHandler.withDefaultResponseURL(ResponseURL);
    bucketCleanerHandler.withDefaultLogGroup(LogGroup);
    bucketCleanerHandler.withDefaultLogStream(LogStream);
    console.log = function () { };
  });
  afterEach(() => {
    AWS.restore();
    console.log = origLog;
  });

  test("Bogus operation fails", () => {
    const bogusType = "bogus";
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason ===
          "Unsupported request type bogus (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb19asdgc092dbbfd)"
        );
      })
      .reply(200);
    return LambdaTester(bucketCleanerHandler.handler)
      .event({
        RequestType: bogusType,
        RequestId: testRequestId,
        ResourceProperties: {},
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create event is a no-op", () => {
    const headBucketFake = sinon.fake.resolves({});
    AWS.mock("S3", "headBucket", headBucketFake);

    const requestType = "Create";
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);
    return LambdaTester(bucketCleanerHandler.handler)
      .event({
        RequestType: requestType,
        RequestId: testRequestId,
        ResourceProperties: {
          BucketName: testBucketName
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        sinon.assert.notCalled(headBucketFake);
        expect(request.isDone()).toBe(true);
      });
  });

  test("Update event is a no-op", () => {
    const headBucketFake = sinon.fake.resolves({});
    AWS.mock("S3", "headBucket", headBucketFake);

    const requestType = "Update";
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);
    return LambdaTester(bucketCleanerHandler.handler)
      .event({
        RequestType: requestType,
        RequestId: testRequestId,
        ResourceProperties: {
          BucketName: testBucketName
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        sinon.assert.notCalled(headBucketFake);
        expect(request.isDone()).toBe(true);
      });
  });

  test("Return early when the bucket is gone", () => {
    const notFoundError = new Error();
    notFoundError.name = "ResourceNotFoundException";

    const headBucketFake = sinon.fake.rejects(notFoundError);
    AWS.mock("S3", "headBucket", headBucketFake);
    const listObjectVersionsFake = sinon.fake.resolves({});
    AWS.mock("S3", "listObjectVersions", listObjectVersionsFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "SUCCESS" &&
          body.PhysicalResourceId === "bucket-cleaner-mockID"
        );
      })
      .reply(200);

    return LambdaTester(bucketCleanerHandler.handler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {
          BucketName: testBucketName
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          headBucketFake,
          sinon.match({
            Bucket: testBucketName
          })
        );
        sinon.assert.notCalled(listObjectVersionsFake);
        expect(request.isDone()).toBe(true);
      });
  });

  test("Return early when the bucket is empty", () => {
    const headBucketFake = sinon.fake.resolves({});
    AWS.mock("S3", "headBucket", headBucketFake);
    const listObjectVersionsFake = sinon.fake.resolves({
      Versions: [],
      DeleteMarkers: []
    });
    AWS.mock("S3", "listObjectVersions", listObjectVersionsFake);
    const deleteObjectsFake = sinon.fake.resolves({});
    AWS.mock("S3", "deleteBucket", deleteObjectsFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "SUCCESS" &&
          body.PhysicalResourceId === "bucket-cleaner-mockID"
        );
      })
      .reply(200);

    return LambdaTester(bucketCleanerHandler.handler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {
          BucketName: testBucketName
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          headBucketFake,
          sinon.match({
            Bucket: testBucketName
          })
        );
        sinon.assert.calledWith(
          listObjectVersionsFake,
          sinon.match({
            Bucket: testBucketName
          })
        );
        sinon.assert.notCalled(deleteObjectsFake);
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete all objects with pagination", () => {
    const headBucketFake = sinon.fake.resolves({});
    AWS.mock("S3", "headBucket", headBucketFake);

    const listObjectVersionsFake = sinon.stub();
    listObjectVersionsFake.onFirstCall().resolves({
      Versions: [
        {
          Key: "mockKey1",
          VersionId: "mockVersionId1"
        },
        {
          Key: "mockKey2",
          VersionId: "mockVersionId2"
        },
      ],
      DeleteMarkers: [
        {
          Key: "mockDeleteMarkerKey1",
          VersionId: "mockDeleteMarkerVersionId1"
        },
      ],
      IsTruncated: true,
      NextKeyMarker: "mockKeyMarker",
      NextVersionIdMarker: "mockNextVersionIdMarker"
    });
    listObjectVersionsFake.resolves({
      Versions: [
        {
          Key: "mockKey3",
          VersionId: "mockVersionId3"
        },
      ],
      DeleteMarkers: [
        {
          Key: "mockDeleteMarkerKey2",
          VersionId: "mockDeleteMarkerVersionId2"
        },
      ]
    });
    AWS.mock("S3", "listObjectVersions", listObjectVersionsFake);

    const deleteObjectsFake = sinon.fake.resolves({
      Errors: []
    });
    AWS.mock("S3", "deleteObjects", deleteObjectsFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "SUCCESS" &&
          body.PhysicalResourceId === "bucket-cleaner-mockID"
        );
      })
      .reply(200);

    return LambdaTester(bucketCleanerHandler.handler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {
          BucketName: testBucketName
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          headBucketFake,
          sinon.match({
            Bucket: testBucketName
          })
        );
        sinon.assert.calledWith(
          listObjectVersionsFake,
          sinon.match({
            Bucket: testBucketName
          })
        );
        sinon.assert.calledWith(
          listObjectVersionsFake,
          sinon.match({
            Bucket: testBucketName,
            KeyMarker: "mockKeyMarker",
            VersionIdMarker: "mockNextVersionIdMarker"
          })
        );
        sinon.assert.calledWith(
          deleteObjectsFake,
          sinon.match({
            Bucket: testBucketName,
            Delete: {
              Objects: [
                {
                  Key: "mockKey1",
                  VersionId: "mockVersionId1"
                },
                {
                  Key: "mockKey2",
                  VersionId: "mockVersionId2"
                },
                {
                  Key: "mockDeleteMarkerKey1",
                  VersionId: "mockDeleteMarkerVersionId1"
                }
              ],
              Quiet: true
            }
          })
        );
        sinon.assert.calledWith(
          deleteObjectsFake,
          sinon.match({
            Bucket: testBucketName,
            Delete: {
              Objects: [
                {
                  Key: "mockKey3",
                  VersionId: "mockVersionId3"
                },
                {
                  Key: "mockDeleteMarkerKey2",
                  VersionId: "mockDeleteMarkerVersionId2"
                }
              ],
              Quiet: true
            }
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Aggregate the delete error", () => {
    const headBucketFake = sinon.fake.resolves({});
    AWS.mock("S3", "headBucket", headBucketFake);

    const listObjectVersionsFake = sinon.fake.resolves({
      Versions: [
        {
          Key: "mockKey1",
          VersionId: "mockVersionId1"
        },
        {
          Key: "mockKey2",
          VersionId: "mockVersionId2"
        },
      ],
      DeleteMarkers: [
        {
          Key: "mockDeleteMarkerKey1",
          VersionId: "mockDeleteMarkerVersionId1"
        },
      ]
    });
    AWS.mock("S3", "listObjectVersions", listObjectVersionsFake);

    const deleteObjectsFake = sinon.fake.resolves({
      Errors: [
        {
          Key: "mockKey1",
          Message: "mockMsg1"
        },
        {
          Key: "mockKey2",
          Message: "mockMsg2"
        }
      ]
    });
    AWS.mock("S3", "deleteObjects", deleteObjectsFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.PhysicalResourceId === "bucket-cleaner-mockID" &&
          body.Reason === "Error: 2/3 objects failed to delete\nError: first failed on key \"mockKey1\": mockMsg1 (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb19asdgc092dbbfd)"
        );
      })
      .reply(200);

    return LambdaTester(bucketCleanerHandler.handler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {
          BucketName: testBucketName
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          headBucketFake,
          sinon.match({
            Bucket: testBucketName
          })
        );
        sinon.assert.calledWith(
          listObjectVersionsFake,
          sinon.match({
            Bucket: testBucketName
          })
        );
        sinon.assert.calledWith(
          deleteObjectsFake,
          sinon.match({
            Bucket: testBucketName,
            Delete: {
              Objects: [
                {
                  Key: "mockKey1",
                  VersionId: "mockVersionId1"
                },
                {
                  Key: "mockKey2",
                  VersionId: "mockVersionId2"
                },
                {
                  Key: "mockDeleteMarkerKey1",
                  VersionId: "mockDeleteMarkerVersionId1"
                }
              ],
              Quiet: true
            }
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });
});
