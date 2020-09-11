// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

describe("Desired count delegation Handler", () => {
  const AWS = require("aws-sdk-mock");
  const sinon = require("sinon");
  const DesiredCountDelegation = require("../lib/desired-count-delegation");
  const LambdaTester = require("lambda-tester").noVersionCheck();
  const nock = require("nock");
  const responseURL = "https://cloudwatch-response-mock.example.com/";
  const testRequestId = "f4ef1b10-c39a-44e3-99c0-fbf7e53c3943";
  let origLog = console.log;

  const testCluster = "mockClusterName";
  const testApp = "mockApp";
  const testEnv = "testEnv";
  const testSvc = "testSvc";
  const testECSService = "testECSService";
  const testNextToken = "mockNextToken";

  beforeEach(() => {
    DesiredCountDelegation.withDefaultResponseURL(responseURL);
    // Prevent logging.
    console.log = function () {};
  });
  afterEach(() => {
    // Restore logger
    AWS.restore();
    console.log = origLog;
  });

  test("invalid operation should return default desired count", () => {
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.Data.DesiredCount == 3;
      })
      .reply(200);

    return LambdaTester(DesiredCountDelegation.handler)
      .event({
        RequestType: "OOPS",
        ResponseURL: responseURL,
        ResourceProperties: {
          DefaultDesiredCount: 3,
        },
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("create operation", () => {
    const getResourcesFake = sinon.fake.resolves({
      ResourceTagMappingList: [],
    });
    const listTasksFake = sinon.stub();
    AWS.mock("ResourceGroupsTaggingAPI", "getResources", getResourcesFake);
    AWS.mock("ECS", "listTasks", listTasksFake);
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.Data.DesiredCount == 3;
      })
      .reply(200);

    return LambdaTester(DesiredCountDelegation.handler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResponseURL: responseURL,
        ResourceProperties: {
          Cluster: testCluster,
          App: testApp,
          Env: testEnv,
          Svc: testSvc,
          DefaultDesiredCount: 3,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          getResourcesFake,
          sinon.match({
            ResourceTypeFilters: ["ecs:service"],
            TagFilters: [
              {
                Key: "copilot-application",
                Values: [testApp],
              },
              {
                Key: "copilot-environment",
                Values: [testEnv],
              },
              {
                Key: "copilot-service",
                Values: [testSvc],
              },
            ],
          })
        );
        sinon.assert.notCalled(listTasksFake);
        expect(request.isDone()).toBe(true);
      });
  });

  test("update operation", () => {
    const getResourcesFake = sinon.fake.resolves({
      ResourceTagMappingList: [
        {
          ResourceARN: testECSService,
        },
      ],
    });
    const listTasksFake = sinon.fake.resolves({
      taskArns: ["mockTask1", "mockTask2"],
    });
    AWS.mock("ResourceGroupsTaggingAPI", "getResources", getResourcesFake);
    AWS.mock("ECS", "listTasks", listTasksFake);
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.Data.DesiredCount == 2;
      })
      .reply(200);

    return LambdaTester(DesiredCountDelegation.handler)
      .event({
        RequestType: "Update",
        RequestId: testRequestId,
        ResponseURL: responseURL,
        ResourceProperties: {
          Cluster: testCluster,
          App: testApp,
          Env: testEnv,
          Svc: testSvc,
          DefaultDesiredCount: 3,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          getResourcesFake,
          sinon.match({
            ResourceTypeFilters: ["ecs:service"],
            TagFilters: [
              {
                Key: "copilot-application",
                Values: [testApp],
              },
              {
                Key: "copilot-environment",
                Values: [testEnv],
              },
              {
                Key: "copilot-service",
                Values: [testSvc],
              },
            ],
          })
        );
        sinon.assert.calledWith(
          listTasksFake,
          sinon.match({
            cluster: testCluster,
            serviceName: testECSService,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("update operation with pagination", () => {
    const getResourcesFake = sinon.fake.resolves({
      ResourceTagMappingList: [
        {
          ResourceARN: testECSService,
        },
      ],
    });
    const listTasksFake = sinon.stub();
    listTasksFake.onCall(0).resolves({
      taskArns: ["mockTask1", "mockTask2"],
      nextToken: testNextToken,
    });
    listTasksFake.onCall(1).resolves({
      taskArns: ["mockTask3", "mockTask4"],
    });
    AWS.mock("ResourceGroupsTaggingAPI", "getResources", getResourcesFake);
    AWS.mock("ECS", "listTasks", listTasksFake);
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.Data.DesiredCount == 4;
      })
      .reply(200);

    return LambdaTester(DesiredCountDelegation.handler)
      .event({
        RequestType: "Update",
        RequestId: testRequestId,
        ResponseURL: responseURL,
        ResourceProperties: {
          Cluster: testCluster,
          App: testApp,
          Env: testEnv,
          Svc: testSvc,
          DefaultDesiredCount: 3,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          getResourcesFake,
          sinon.match({
            ResourceTypeFilters: ["ecs:service"],
            TagFilters: [
              {
                Key: "copilot-application",
                Values: [testApp],
              },
              {
                Key: "copilot-environment",
                Values: [testEnv],
              },
              {
                Key: "copilot-service",
                Values: [testSvc],
              },
            ],
          })
        );
        sinon.assert.calledWith(
          listTasksFake.firstCall,
          sinon.match({
            cluster: testCluster,
            serviceName: testECSService,
          })
        );
        sinon.assert.calledWith(
          listTasksFake.secondCall,
          sinon.match({
            cluster: testCluster,
            serviceName: testECSService,
            nextToken: testNextToken,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("delete operation should do nothing", () => {
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(DesiredCountDelegation.handler)
      .event({
        RequestType: "Delete",
        ResponseURL: responseURL,
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });
});
