// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";
const { mockClient } = require('aws-sdk-client-mock');
const { ECSClient, DescribeServicesCommand } = require("@aws-sdk/client-ecs");
const { SQSClient, GetQueueUrlCommand, GetQueueAttributesCommand } = require("@aws-sdk/client-sqs");
const lambdaTester = require("lambda-tester").noVersionCheck();
const sinon = require("sinon");
const calculatorLambda = require("../lib/backlog-per-task-calculator");

describe("BacklogPerTask metric calculator", () => {
  const origConsole = console;
  const origEnvVars = process.env;
  let ecsMock, sqsMock;

  beforeAll(() => {
    jest
    .spyOn(global.Date, 'now')
    .mockImplementation(() =>
      new Date('2021-09-02').valueOf(), // maps to 1630540800000.
    );
    ecsMock = mockClient(ECSClient);
    sqsMock = mockClient(SQSClient);
  });

  beforeEach(() => {
    ecsMock.reset();
    sqsMock.reset();
    process.env = { ...origEnvVars };
    console.error = sinon.stub();
    console.log = sinon.stub();
  });

  afterEach(() => {
    process.env = origEnvVars;
  });

  afterAll(() => {
    console = origConsole;
    jest.spyOn(global.Date, 'now').mockClear();
    ecsMock.restore();
    sqsMock.restore();
  });

  test("should write the error to console on unexpected failure", async () => {
    // GIVEN
    console.error = sinon.stub();
    console.log = sinon.stub()
    ecsMock.on(DescribeServicesCommand).rejects("some message");

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

    ecsMock.on(DescribeServicesCommand).resolves({
      services: [
        {
          runningCount: 0,
        },
      ],
    });
    sqsMock.on(GetQueueUrlCommand).resolves({
      QueueUrl: "url",
    });
    sqsMock.on(GetQueueAttributesCommand).resolves({
      Attributes: {
        ApproximateNumberOfMessages: 100,
      },
    });

    // WHEN
    const tester = lambdaTester(calculatorLambda.handler).event({});

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

    ecsMock.on(DescribeServicesCommand).resolves({
      services: [
        {
          runningCount: 3,
        },
      ],
    });

    sqsMock.on(GetQueueUrlCommand).resolves({
      QueueUrl: "url",
    });

    sqsMock.on(GetQueueAttributesCommand, { QueueUrl: 'url' })
      .resolvesOnce({
        Attributes: {
          ApproximateNumberOfMessages: 100,
        },
      })
      .resolvesOnce({
        Attributes: {
          ApproximateNumberOfMessages: 495,
        },
      });

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