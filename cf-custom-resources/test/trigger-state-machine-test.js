// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

describe("trigger state machine", () => {
  const aws = require("aws-sdk-mock");
  const lambdaTester = require("lambda-tester").noVersionCheck();
  const nock = require("nock");
  const sinon = require("sinon");
  const handler = require("../lib/trigger-state-machine");

  const responseURL = "https://cloudwatch-response-mock.example.com/";
  const logGroup = "/aws/lambda/testLambda";
  const logStream = "2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd";
  const testRequestId = "f4ef1b10-c39a-44e3-99c0-fbf7e53c3943";

  const stateMachineARN = "arn::mock::statemachine";

  const origConsole = console;

  handler.withDeadlineExpired(() => {
    return new Promise((resolve, reject) => { });
  });

  afterEach(() => {
    aws.restore();
  });
  afterAll(() => {
    console = origConsole;
  });

  test("bogus operation fails", () => {
    console.error = () => { };
    const request = nock(responseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason ===
          "Unsupported request type bogus (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd)"
        );
      })
      .reply(200);
    return lambdaTester(handler.handler)
      .context({
        logGroupName: logGroup,
        logStreamName: logStream,
      })
      .event({
        ResponseURL: responseURL,
        RequestType: "bogus",
        RequestId: testRequestId,
        ResourceProperties: {},
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("delete event is a no-op", () => {
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.PhysicalResourceId === "randomID";
      })
      .reply(200);
    return lambdaTester(handler.handler)
      .context({
        logGroupName: logGroup,
        logStreamName: logStream,
      })
      .event({
        ResponseURL: responseURL,
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {},
        LogicalResourceId: "mockID",
        PhysicalResourceId: "randomID",
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("happy path", () => {
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" &&
          body.PhysicalResourceId === "mockID"; // reuses logical ID if no physical ID
      })
      .reply(200);

    const fake = sinon.fake.resolves({ status: "SUCCEEDED" });
    aws.mock("StepFunctions", "startSyncExecution", fake);

    return lambdaTester(handler.handler)
      .context({
        logGroupName: logGroup,
        logStreamName: logStream,
      })
      .event({
        ResponseURL: responseURL,
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          StateMachineARN: stateMachineARN,
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
        sinon.assert.calledWith(fake, {
          stateMachineArn: stateMachineARN,
        });
      });
  });

  test("state machine failure", () => {
    console.error = () => { };
    const request = nock(responseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason ===
          "State machine failed: some error (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd)" &&
          body.PhysicalResourceId === "physicalID"
        );
      })
      .reply(200);

    const fake = sinon.fake.resolves({ status: "FAILED", cause: "some error" });
    aws.mock("StepFunctions", "startSyncExecution", fake);

    return lambdaTester(handler.handler)
      .context({
        logGroupName: logGroup,
        logStreamName: logStream,
      })
      .event({
        ResponseURL: responseURL,
        RequestType: "Update",
        RequestId: testRequestId,
        ResourceProperties: {
          StateMachineARN: stateMachineARN,
        },
        LogicalResourceId: "mockID",
        PhysicalResourceId: "physicalID",
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
        sinon.assert.calledWith(fake, {
          stateMachineArn: stateMachineARN,
        });
      });
  });

  test("sdk error", () => {
    console.error = () => { };
    const request = nock(responseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason ===
          "some error (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd)" &&
          body.PhysicalResourceId === "mockID"
        );
      })
      .reply(200);

    const fake = sinon.fake.rejects("some error");
    aws.mock("StepFunctions", "startSyncExecution", fake);

    return lambdaTester(handler.handler)
      .context({
        logGroupName: logGroup,
        logStreamName: logStream,
      })
      .event({
        ResponseURL: responseURL,
        RequestType: "Update",
        RequestId: testRequestId,
        ResourceProperties: {
          StateMachineARN: stateMachineARN,
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
        sinon.assert.calledWith(fake, {
          stateMachineArn: stateMachineARN,
        });
      });
  });
});
