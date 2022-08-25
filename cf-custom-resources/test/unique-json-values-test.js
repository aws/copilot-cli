// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

describe("Unique Aliases", () => {
  const LambdaTester = require("lambda-tester").noVersionCheck();
  const uniqueJSONValues = require("../lib/unique-json-values");
  const nock = require("nock");
  const responseURL = "https://cloudwatch-response-mock.example.com/";
  const logGroup = "/aws/lambda/testLambda";
  const logStream = "2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd";

  let origLog = console.log;
  let origError = console.error;

  const testRequestId = "f4ef1b10-c39a-44e3-99c0-fbf7e53c3943";

  beforeEach(() => {
    console.log = function () { };
    console.error = function () { };
  });
  afterEach(() => {
    console.log = origLog;
    console.error = origError;
  });

  test("Bogus operation fails", () => {
    const request = nock(responseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason ===
          "Unsupported request type bogus (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd)"
        );
      })
      .reply(200);
    return LambdaTester(uniqueJSONValues.handler)
      .context({
        logGroupName: logGroup,
        logStreamName: logStream
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

  test("Delete event is a no-op", () => {
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);
    return LambdaTester(uniqueJSONValues.handler)
      .context({
        logGroupName: logGroup,
        logStreamName: logStream
      })
      .event({
        ResponseURL: responseURL,
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {},
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  const aliasTest = (name, input, expectedOutput) => {
    const tt = (name, reqType, input, expectedOutput) => {
      test(name, () => {
        const request = nock(responseURL)
          .put("/", (body) => {
            return body.Status === "SUCCESS" &&
              body.PhysicalResourceId === "mockID" &&
              JSON.stringify(body.Data.UniqueValues) === JSON.stringify(expectedOutput);
          })
          .reply(200);

        return LambdaTester(uniqueJSONValues.handler)
          .context({
            logGroupName: logGroup,
            logStreamName: logStream
          })
          .event({
            ResponseURL: responseURL,
            RequestType: reqType,
            RequestId: testRequestId,
            ResourceProperties: {
              Aliases: JSON.stringify(input), // aliases get passed as a string
            },
            LogicalResourceId: "mockID",
          })
          .expectResolve(() => {
            expect(request.isDone()).toBe(true);
          });
      });
    };

    tt(`Create/${name}`, "Create", input, expectedOutput);
    tt(`Update/${name}`, "Update", input, expectedOutput);
  };

  aliasTest("no aliases", {}, []);

  aliasTest("one service", {
    "svc1": ["svc1.com", "example.com"],
  }, ["example.com", "svc1.com"]);

  aliasTest("two services no common aliases", {
    "svc1": ["svc1.com"],
    "svc2": ["svc2.com"]
  }, ["svc1.com", "svc2.com"]);

  aliasTest("two services, one with multiple common aliases", {
    "svc1": ["svc1.com"],
    "svc2": ["svc2.com", "example.com"]
  }, ["example.com", "svc1.com", "svc2.com"]);

  aliasTest("two services with a common alias", {
    "svc1": ["svc1.com", "example.com"],
    "svc2": ["svc2.com", "example.com"]
  }, ["example.com", "svc1.com", "svc2.com"]);
});