// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

describe("DNS Validated Certificate Handler", () => {
  const AWS = require("aws-sdk-mock");
  const LambdaTester = require("lambda-tester").noVersionCheck();
  const sinon = require("sinon");
  const handler = require("../lib/custom-domain");
  const nock = require("nock");
  const ResponseURL = "https://cloudwatch-response-mock.example.com/";

  let origLog = console.log;
  const testAppName = "myapp";
  const testEnvName = "test";
  const testDomainName = "example.com";
  const testAliases = `{"frontend": "v1.${testEnvName}.${testAppName}.${testDomainName},foobar.com"}`;
  const testLoadBalancerDNS =
    "examp-publi-gsedbvf8t12c-852245110.us-west-1.elb.amazonaws.com.";
  const testLBHostedZone = "Z1H1FL5HABSF5";
  const testHostedZoneId = "Z3P5QSUBK4POTI";
  const testRootDNSRole = "mockRole";

  beforeEach(() => {
    handler.withDefaultResponseURL(ResponseURL);
    handler.withWaiter(function () {
      // Mock waiter is merely a self-fulfilling promise
      return {
        promise: () => {
          return new Promise((resolve) => {
            resolve();
          });
        },
      };
    });
    console.log = function () {};
  });
  afterEach(() => {
    // Restore waiters and logger
    handler.reset();
    AWS.restore();
    console.log = origLog;
  });

  test("Empty event payload fails", () => {
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason === "Unsupported request type undefined"
        );
      })
      .reply(200);
    return LambdaTester(handler.handler)
      .event({
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
        },
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("Bogus operation fails", () => {
    const bogusType = "bogus";
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason === "Unsupported request type " + bogusType
        );
      })
      .reply(200);
    return LambdaTester(handler.handler)
      .event({
        RequestType: bogusType,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
        },
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("Error if failed to parse aliases", () => {
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason === "Cannot parse badAliases into JSON format."
        );
      })
      .reply(200);
    return LambdaTester(handler.handler)
      .event({
        RequestType: "Create",
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          Aliases: "badAliases",
          Region: "us-east-1",
          LoadBalancerDNS: testLoadBalancerDNS,
          LoadBalancerHostedZone: testLBHostedZone,
          AppDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("Error if cannot find any domain hosted zone for an alias", () => {
    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [],
    });

    AWS.mock("Route53", "listHostedZonesByName", listHostedZonesByNameFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason ===
            "Couldn't find any Hosted Zone with DNS name test.myapp.example.com."
        );
      })
      .reply(200);
    return LambdaTester(handler.handler)
      .event({
        RequestType: "Create",
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          Aliases: testAliases,
          Region: "us-east-1",
          LoadBalancerDNS: testLoadBalancerDNS,
          LoadBalancerHostedZone: testLBHostedZone,
          AppDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: `${testEnvName}.${testAppName}.${testDomainName}`,
            MaxItems: "1",
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Upsert success", () => {
    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [
        {
          Id: `/hostedzone/${testHostedZoneId}`,
        },
      ],
    });

    AWS.mock(
      "Route53",
      "changeResourceRecordSets",
      changeResourceRecordSetsFake
    );
    AWS.mock("Route53", "listHostedZonesByName", listHostedZonesByNameFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);
    return LambdaTester(handler.handler)
      .event({
        RequestType: "Create",
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          Aliases: testAliases,
          Region: "us-east-1",
          LoadBalancerDNS: testLoadBalancerDNS,
          LoadBalancerHostedZone: testLBHostedZone,
          AppDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: `${testEnvName}.${testAppName}.${testDomainName}`,
            MaxItems: "1",
          })
        );
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: {
              Changes: [
                {
                  Action: "UPSERT",
                  ResourceRecordSet: {
                    Name: `v1.${testEnvName}.${testAppName}.${testDomainName}`,
                    Type: "A",
                    AliasTarget: {
                      HostedZoneId: testLBHostedZone,
                      DNSName: testLoadBalancerDNS,
                      EvaluateTargetHealth: true,
                    },
                  },
                },
              ],
            },
            HostedZoneId: testHostedZoneId,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete success", () => {
    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [
        {
          Id: `/hostedzone/${testHostedZoneId}`,
        },
      ],
    });

    AWS.mock(
      "Route53",
      "changeResourceRecordSets",
      changeResourceRecordSetsFake
    );
    AWS.mock("Route53", "listHostedZonesByName", listHostedZonesByNameFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);
    return LambdaTester(handler.handler)
      .event({
        RequestType: "Delete",
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          Aliases: testAliases,
          Region: "us-east-1",
          LoadBalancerDNS: testLoadBalancerDNS,
          LoadBalancerHostedZone: testLBHostedZone,
          AppDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        // use cached result
        sinon.assert.notCalled(listHostedZonesByNameFake);
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: {
              Changes: [
                {
                  Action: "DELETE",
                  ResourceRecordSet: {
                    Name: `v1.${testEnvName}.${testAppName}.${testDomainName}`,
                    Type: "A",
                    AliasTarget: {
                      HostedZoneId: testLBHostedZone,
                      DNSName: testLoadBalancerDNS,
                      EvaluateTargetHealth: true,
                    },
                  },
                },
              ],
            },
            HostedZoneId: testHostedZoneId,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });
});
