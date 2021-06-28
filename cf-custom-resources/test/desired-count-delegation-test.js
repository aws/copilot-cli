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
    const describeServicesFake = sinon.stub();
    AWS.mock("ResourceGroupsTaggingAPI", "getResources", getResourcesFake);
    AWS.mock("ECS", "describeServices", describeServicesFake);
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.Data.DesiredCount == 3 && body.PhysicalResourceId === "copilot/apps/mockApp/envs/testEnv/services/testSvc/autoscaling";
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
        sinon.assert.notCalled(describeServicesFake);
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
    const describeServicesFake = sinon.fake.resolves({
      services: [
        {
          desiredCount: 2,
        },
      ],
    });
    AWS.mock("ResourceGroupsTaggingAPI", "getResources", getResourcesFake);
    AWS.mock("ECS", "describeServices", describeServicesFake);
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
        PhysicalResourceId: "mockID"
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
          describeServicesFake,
          sinon.match({
            cluster: testCluster,
            services: [testECSService],
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
        PhysicalResourceId: "mockID"
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });
});
