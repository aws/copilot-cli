// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

describe("ALB Rule Priority Generator", () => {
  const AWS = require("aws-sdk-mock");
  const LambdaTester = require("lambda-tester").noVersionCheck();
  const sinon = require("sinon");
  const albRulePriorityHandler = require("../lib/alb-rule-priority-generator");
  const nock = require("nock");
  const ResponseURL = "https://cloudwatch-response-mock.example.com/";
  const LogGroup = "/aws/lambda/testLambda";
  const LogStream = "2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd";

  let origLog = console.log;

  const testRequestId = "f4ef1b10-c39a-44e3-99c0-fbf7e53c3943";
  const testALBListenerArn =
    "arn:aws:elasticloadbalancing:us-west-2:00000000:listener/app/lblistner";

  beforeEach(() => {
    albRulePriorityHandler.withDefaultResponseURL(ResponseURL);
    albRulePriorityHandler.withDefaultLogGroup(LogGroup);
    albRulePriorityHandler.withDefaultLogStream(LogStream);
    console.log = function () {};
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
            "Unsupported request type bogus (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd)"
        );
      })
      .reply(200);
    return LambdaTester(albRulePriorityHandler.nextAvailableRulePriorityHandler)
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

  test("Delete event is a no-op", () => {
    const describeRulesFake = sinon.fake.resolves({
      Rules: [],
    });

    AWS.mock("ELBv2", "describeRules", describeRulesFake);

    const requestType = "Delete";
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);
    return LambdaTester(albRulePriorityHandler.nextAvailableRulePriorityHandler)
      .event({
        RequestType: requestType,
        RequestId: testRequestId,
        ResourceProperties: {},
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        sinon.assert.notCalled(describeRulesFake);
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation returns non-root rule priority 1 when only the default rule is present", () => {
    const describeRulesFake = sinon.fake.resolves({
      Rules: [
        {
          Priority: "default",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
      ],
    });

    AWS.mock("ELBv2", "describeRules", describeRulesFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "SUCCESS" &&
          body.Data.Priority == 1 &&
          body.PhysicalResourceId === "alb-rule-priority-mockID"
        );
      })
      .reply(200);

    return LambdaTester(albRulePriorityHandler.nextAvailableRulePriorityHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          ListenerArn: testALBListenerArn,
          RulePath: ["/api"],
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeRulesFake,
          sinon.match({
            ListenerArn: testALBListenerArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Update operation returns root rule priority 50000 when only the default rule is present", () => {
    const describeRulesFake = sinon.fake.resolves({
      Rules: [
        {
          Priority: "default",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
      ],
    });

    AWS.mock("ELBv2", "describeRules", describeRulesFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "SUCCESS" &&
          body.Data.Priority == 50000 &&
          body.PhysicalResourceId === "alb-rule-priority-mockID"
        );
      })
      .reply(200);

    return LambdaTester(albRulePriorityHandler.nextAvailableRulePriorityHandler)
      .event({
        RequestType: "Update",
        RequestId: testRequestId,
        ResourceProperties: {
          ListenerArn: testALBListenerArn,
          RulePath: ["/"],
        },
        LogicalResourceId: "mockID",
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeRulesFake,
          sinon.match({
            ListenerArn: testALBListenerArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation returns non-root rule priority 1 when any root rule is present", () => {
    const describeRulesFake = sinon.fake.resolves({
      Rules: [
        {
          Priority: "50000",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: false,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
      ],
    });

    AWS.mock("ELBv2", "describeRules", describeRulesFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.Data.Priority == 1;
      })
      .reply(200);

    return LambdaTester(albRulePriorityHandler.nextAvailableRulePriorityHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          ListenerArn: testALBListenerArn,
          RulePath: ["/api"],
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeRulesFake,
          sinon.match({
            ListenerArn: testALBListenerArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation returns non-root rule priority max + 1", () => {
    // This set of rules has the default, 3 and 5 rule priorities. We don't try to fill
    // in the gaps, we just create one that is 1 + the max. In this case, 6.
    const describeRulesFake = sinon.fake.resolves({
      Rules: [
        {
          Priority: "default",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
        {
          Priority: "3",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
        {
          Priority: "5",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
      ],
    });

    AWS.mock("ELBv2", "describeRules", describeRulesFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.Data.Priority == 6;
      })
      .reply(200);

    return LambdaTester(albRulePriorityHandler.nextAvailableRulePriorityHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          ListenerArn: testALBListenerArn,
          RulePath: ["/api"],
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeRulesFake,
          sinon.match({
            ListenerArn: testALBListenerArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation returns root rule priority min - 1", () => {
    const describeRulesFake = sinon.fake.resolves({
      Rules: [
        {
          Priority: "default",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
        {
          Priority: "49999",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
        {
          Priority: "50000",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
      ],
    });

    AWS.mock("ELBv2", "describeRules", describeRulesFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.Data.Priority == 49998;
      })
      .reply(200);

    return LambdaTester(albRulePriorityHandler.nextAvailableRulePriorityHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          ListenerArn: testALBListenerArn,
          RulePath: ["/"],
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeRulesFake,
          sinon.match({
            ListenerArn: testALBListenerArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation returns non-root rule priority max + 1 for paginated response", () => {
    // This set of rules has the default, 3 and 5 rule priorities. We don't try to fill
    // in the gaps, we just create one that is 1 + the max. In this case, 6.
    const describeRulesFake = sinon.stub();
    const testNextMarkerToken = "12345";
    describeRulesFake.onCall(0).resolves({
      NextMarker: testNextMarkerToken,
      Rules: [
        {
          Priority: "default",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
      ],
    });

    describeRulesFake.onCall(1).resolves({
      Rules: [
        {
          Priority: "100",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
        {
          Priority: "48000",
          Conditions: [],
          RuleArn:
            "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
      ],
    });

    AWS.mock("ELBv2", "describeRules", describeRulesFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS" && body.Data.Priority == 101;
      })
      .reply(200);

    return LambdaTester(albRulePriorityHandler.nextAvailableRulePriorityHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          ListenerArn: testALBListenerArn,
          RulePath: ["/api"],
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeRulesFake.firstCall,
          sinon.match({
            ListenerArn: testALBListenerArn,
          })
        );

        sinon.assert.calledWith(
          describeRulesFake.secondCall,
          sinon.match({
            ListenerArn: testALBListenerArn,
            Marker: testNextMarkerToken,
          })
        );

        expect(request.isDone()).toBe(true);
      });
  });
  test("Create operation returns non-root rule priority max + number of rules in case of multiple ports", () => {
    // This set of rules has the default, 3 and 5 rule priorities. We don't try to fill
    // in the gaps, we just create one that is 1 + the max. In this case, 6.
    const describeRulesFake = sinon.fake.resolves({
      Rules: [
        {
          Priority: "default",
          Conditions: [],
          RuleArn:
              "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                  "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
        {
          Priority: "3",
          Conditions: [],
          RuleArn:
              "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                  "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
        {
          Priority: "5",
          Conditions: [],
          RuleArn:
              "arn:aws:elasticloadbalancing:us-west-2:000000000:listener-rule/app/rule",
          IsDefault: true,
          Actions: [
            {
              TargetGroupArn:
                  "arn:aws:elasticloadbalancing:us-west-2:000000000:targetgroup/tg",
              Type: "forward",
            },
          ],
        },
      ],
    });

    AWS.mock("ELBv2", "describeRules", describeRulesFake);
    const request = nock(ResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" &&
              body.Data.Priority == 6 &&
              body.Data.Priority2 == 7 &&
              body.Data.Priority1 == 50000;
        })
        .reply(200);

    return LambdaTester(albRulePriorityHandler.nextAvailableRulePriorityHandler)
        .event({
          RequestType: "Create",
          RequestId: testRequestId,
          ResourceProperties: {
            ListenerArn: testALBListenerArn,
            RulePath: ["/api", "/", "admin"],
          },
        })
        .expectResolve(() => {
          sinon.assert.calledWith(
              describeRulesFake,
              sinon.match({
                ListenerArn: testALBListenerArn,
              })
          );
          expect(request.isDone()).toBe(true);
        });
  });

});
