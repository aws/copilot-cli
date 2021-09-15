// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";
const aws = require("aws-sdk-mock");
const lambdaTester = require("lambda-tester").noVersionCheck();
const sinon = require("sinon");
const calculatorLambda = require("../lib/backlog-per-task-calculator");


describe("BacklogPerTask metric calculator", () => {
  const origConsole = console;
  const origEnvVars = process.env;

  beforeAll(() => {
    jest
      .spyOn(global.Date, 'now')
      .mockImplementation(() =>
        new Date('2021-09-02').valueOf(), // maps to 1630540800000.
      );
  });

  afterEach(() => {
    process.env = origEnvVars;
    aws.restore();
  });

  afterAll(() => {
    console = origConsole;
    jest.spyOn(global.Date, 'now').mockClear();
  });

  test("should write the error to console on unexpected failure", async () => {
    // GIVEN
    console.error = sinon.stub();
    console.log = sinon.stub()
    aws.mock("ECS", "describeServices", sinon.fake.rejects("some message"));


    // WHEN
    const tester = lambdaTester(calculatorLambda.handler)
      .event({});

    // THEN
    await tester.expectResolve(() => {
        sinon.assert.called(console.error);
        sinon.assert.calledWith(console.error, "Unexpected error Error: some message");
        sinon.assert.notCalled(console.log);
      });
  });

  test("should write the total number of messages to console.log if there are no tasks running", async () => {
    // GIVEN
    process.env = {
      ...process.env,
      NAMESPACE: "app-env-service",
      CLUSTER_NAME: "cluster",
      SERVICE_NAME: "service",
      QUEUE_NAMES: "queue1",
    }
    console.error = sinon.stub();
    console.log = sinon.stub();

    aws.mock("ECS", "describeServices", sinon.fake.resolves({
      services: [
        {
          runningCount: 0,
        },
      ],
    }));
    aws.mock("SQS", "getQueueUrl", sinon.fake.resolves({
      QueueUrl: "url",
    }));
    aws.mock("SQS", "getQueueAttributes", sinon.fake.resolves({
      Attributes: {
        ApproximateNumberOfMessages: 100,
      },
    }));


    // WHEN
    const tester = lambdaTester(calculatorLambda.handler)
      .event({});

    // THEN
    await tester.expectResolve(() => {
      sinon.assert.called(console.log);
      sinon.assert.calledWith(console.log, JSON.stringify({
        "_aws": {
          "Timestamp": 1630540800000,
          "CloudWatchMetrics": [{
            "Namespace": "app-env-service",
            "Dimensions": [["QueueName"]],
            "Metrics": [{"Name":"BacklogPerTask", "Unit": "Count"}]
          }],
        },
        "QueueName": "queue1",
        "BacklogPerTask": 100,
      }));
      sinon.assert.notCalled(console.error);
    });
  });

  test("should write the backlog per task for each queue", async () => {
    // GIVEN
    process.env = {
      ...process.env,
      NAMESPACE: "app-env-service",
      CLUSTER_NAME: "cluster",
      SERVICE_NAME: "service",
      QUEUE_NAMES: "queue1,queue2",
    }
    console.error = sinon.stub();
    console.log = sinon.stub();

    aws.mock("ECS", "describeServices", sinon.fake.resolves({
      services: [
        {
          runningCount: 3,
        },
      ],
    }));
    aws.mock("SQS", "getQueueUrl", sinon.fake.resolves({
      QueueUrl: "url",
    }));
    aws.mock("SQS", "getQueueAttributes", sinon.stub()
      .onFirstCall().resolves({
        Attributes: {
          ApproximateNumberOfMessages: 100,
        },
      })
      .onSecondCall().resolves({
        Attributes: {
          ApproximateNumberOfMessages: 495,
        },
      }));


    // WHEN
    const tester = lambdaTester(calculatorLambda.handler)
      .event({});

    // THEN
    await tester.expectResolve(() => {
      sinon.assert.called(console.log);
      sinon.assert.calledWith(console.log.firstCall, JSON.stringify({
        "_aws": {
          "Timestamp": 1630540800000,
          "CloudWatchMetrics": [{
            "Namespace": "app-env-service",
            "Dimensions": [["QueueName"]],
            "Metrics": [{"Name":"BacklogPerTask", "Unit": "Count"}]
          }],
        },
        "QueueName": "queue1",
        "BacklogPerTask": 34,
      }));
      sinon.assert.calledWith(console.log.secondCall, JSON.stringify({
        "_aws": {
          "Timestamp": 1630540800000,
          "CloudWatchMetrics": [{
            "Namespace": "app-env-service",
            "Dimensions": [["QueueName"]],
            "Metrics": [{"Name":"BacklogPerTask", "Unit": "Count"}]
          }],
        },
        "QueueName": "queue2",
        "BacklogPerTask": 165,
      }));
      sinon.assert.notCalled(console.error);
    });
  });
});