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
    console.log = function () {};
    console.error = function () {};
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

  test("Delete event is a no-op", () => {
    const request = nock(responseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);
    return LambdaTester(uniqueJSONValues.handler)
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
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  const aliasTest = (name, props, expectedOutput) => {
    const tt = (name, reqType, props, expectedOutput) => {
      test(name, () => {
        const request = nock(responseURL)
          .put("/", (body) => {
            return (
              body.Status === "SUCCESS" &&
              body.PhysicalResourceId === "mockID" &&
              JSON.stringify(body.Data.UniqueValues) ===
                JSON.stringify(expectedOutput)
            );
          })
          .reply(200);

        return LambdaTester(uniqueJSONValues.handler)
          .context({
            logGroupName: logGroup,
            logStreamName: logStream,
          })
          .event({
            ResponseURL: responseURL,
            RequestType: reqType,
            RequestId: testRequestId,
            ResourceProperties: props,
            LogicalResourceId: "mockID",
          })
          .expectResolve(() => {
            expect(request.isDone()).toBe(true);
          });
      });
    };

    tt(`Create/${name}`, "Create", props, expectedOutput);
    tt(`Update/${name}`, "Update", props, expectedOutput);
  };

  aliasTest(
    "no aliases",
    {
      Aliases: "",
      FilterFor: "",
    },
    []
  );

  aliasTest(
    "one service",
    {
      Aliases: JSON.stringify({
        svc1: ["svc1.com", "example.com"],
      }),
      FilterFor: "svc1",
    },
    ["example.com", "svc1.com"]
  );

  aliasTest(
    "one service excluded",
    {
      Aliases: JSON.stringify({
        svc1: ["svc1.com", "example.com"],
      }),
      FilterFor: "svc2",
    },
    []
  );

  aliasTest(
    "one service empty filter for",
    {
      Aliases: JSON.stringify({
        svc1: ["svc1.com", "example.com"],
      }),
      FilterFor: "",
    },
    []
  );

  aliasTest(
    "two services no common aliases",
    {
      Aliases: JSON.stringify({
        svc1: ["svc1.com"],
        svc2: ["svc2.com"],
      }),
      FilterFor: "svc1,svc2",
    },
    ["svc1.com", "svc2.com"]
  );

  aliasTest(
    "two services, one with multiple aliases",
    {
      Aliases: JSON.stringify({
        svc1: ["svc1.com"],
        svc2: ["svc2.com", "example.com"],
      }),
      FilterFor: "svc1,svc2",
    },
    ["example.com", "svc1.com", "svc2.com"]
  );

  aliasTest(
    "two services with a common alias",
    {
      Aliases: JSON.stringify({
        svc1: ["svc1.com", "example.com"],
        svc2: ["svc2.com", "example.com"],
      }),
      FilterFor: "svc1,svc2",
    },
    ["example.com", "svc1.com", "svc2.com"]
  );

  aliasTest(
    "three services with a common alias one service filtered out",
    {
      Aliases: JSON.stringify({
        svc1: ["svc1.com", "example.com"],
        svc2: ["svc2.com", "example.com", "example2.com"],
        svc3: ["svc3.com", "example.com", "example2.com"],
      }),
      FilterFor: "svc2,svc1",
    },
    ["example.com", "example2.com", "svc1.com", "svc2.com"]
  );

  aliasTest(
    "bunch of services with single alias, some filtered out, out of order",
    {
      Aliases: JSON.stringify({
        lbws3: ["three.lbws.com"],
        backend1: ["one.backend.internal"],
        lbws1: ["one.lbws.com"],
        lbws4: ["four.lbws.com"],
        backend2: ["two.backend.internal"],
        lbws2: ["two.lbws.com"],
        lbws6: ["lbws.com"],
        lbws5: ["lbws.com"],
      }),
      FilterFor: "lbws2,lbws3,lbws4,lbws1,lbws5,lbws6",
      AdditionalStrings: ["lbws.com"],
    },
    [
      "four.lbws.com",
      "lbws.com",
      "one.lbws.com",
      "three.lbws.com",
      "two.lbws.com",
    ]
  );

  aliasTest(
    "bunch of services with single alias, some FilterFor services don't exist",
    {
      Aliases: JSON.stringify({
        lbws1: ["lbws1.com"],
        lbws2: ["lbws2.com"],
        lbws3: ["lbws3.com"],
        lbws4: ["lbws4.com"],
      }),
      FilterFor: "lbws1,lbws5,lbws6,lbws3",
    },
    ["lbws1.com", "lbws3.com"]
  );

  aliasTest(
    "bunch of services with single alias, and additional alias",
    {
      Aliases: JSON.stringify({
        lbws1: ["lbws1.com"],
        lbws2: ["lbws2.com"],
        lbws3: ["lbws3.com"],
      }),
      FilterFor: "lbws1",
      AdditionalStrings: ["example.com", "foobar.com"],
    },
    ["example.com", "foobar.com", "lbws1.com"]
  );
});
