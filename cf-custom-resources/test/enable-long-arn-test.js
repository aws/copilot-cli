// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const AWS = require("aws-sdk-mock");
const sinon = require("sinon");
const LongArnsLambda = require("../lib/enable-long-arns");
const LambdaTester = require("lambda-tester").noVersionCheck();
const nock = require("nock");

describe("Enable Long ARN Handler", () => {
  const responseURL = "https://cloudwatch-response-mock.example.com/";
  const testRequestId = "f4ef1b10-c39a-44e3-99c0-fbf7e53c3943";

  beforeEach(() => {
    AWS.restore();
  });

  test("create operation", () => {
    // GIVEN
    const mockPutAccountSetting = sinon.stub();
    mockPutAccountSetting.onFirstCall().resolves(null);
    mockPutAccountSetting.onSecondCall().resolves(null);
    mockPutAccountSetting.onThirdCall().resolves(null);

    AWS.mock("ECS", "putAccountSetting", mockPutAccountSetting);
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
      })
      .reply(200);

    // WHEN
    const lambda = LambdaTester(LongArnsLambda.handler).event({
      RequestType: "Create",
      RequestId: testRequestId,
      ResponseURL: responseURL,
      LogicalResourceId: "mockID"
    });

    // THEN
    lambda.expectResolve(() => {
      sinon.assert.calledWith(
        mockPutAccountSetting.firstCall,
        sinon.match({
          name: "serviceLongArnFormat",
          value: "enabled",
        })
      );
      sinon.assert.calledWith(
        mockPutAccountSetting.secondCall,
        sinon.match({
          name: "taskLongArnFormat",
          value: "enabled",
        })
      );
      sinon.assert.calledWith(
        mockPutAccountSetting.thirdCall,
        sinon.match({
          name: "containerInstanceLongArnFormat",
          value: "enabled",
        })
      );
      expect(request.isDone()).toBe(true);
    });
  });

  test("update operation should do nothing", () => {
    // GIVEN
    const mockPutAccountSetting = sinon.stub();

    AWS.mock("ECS", "putAccountSetting", mockPutAccountSetting);
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
      })
      .reply(200);

    // WHEN
    const lambda = LambdaTester(LongArnsLambda.handler).event({
      RequestType: "Update",
      RequestId: testRequestId,
      ResponseURL: responseURL,
      PhysicalResourceId: "mockID"
    });

    // THEN
    lambda.expectResolve(() => {
      sinon.assert.notCalled(mockPutAccountSetting);
      expect(request.isDone()).toBe(true);
    });
  });

  test("delete operation should do nothing", () => {
    // GIVEN
    const mockPutAccountSetting = sinon.stub();

    AWS.mock("ECS", "putAccountSetting", mockPutAccountSetting);
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    // WHEN
    const lambda = LambdaTester(LongArnsLambda.handler).event({
      RequestType: "Delete",
      RequestId: testRequestId,
      ResponseURL: responseURL,
    });

    // THEN
    lambda.expectResolve(() => {
      sinon.assert.notCalled(mockPutAccountSetting);
      expect(request.isDone()).toBe(true);
    });
  });
});
