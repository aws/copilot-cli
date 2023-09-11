// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const AWS = require("aws-sdk-mock");
const LambdaTester = require("lambda-tester").noVersionCheck();
const sinon = require("sinon");
const nock = require("nock");
let origLog = console.log;

const { attemptsValidationOptionsReady } = require("../lib/wkld-custom-domain");

describe("DNS Certificate Validation And Custom Domains for NLB", () => {
  // Mock requests.
  const mockServiceName = "web";
  const mockEnvName = "mockEnv";
  const mockAppName = "mockApp";
  const mockDomainName = "mockDomain.com";
  const mockEnvHostedZoneID = "mockEnvHostedZoneID";
  const mockLBDNS = "mockLBDNS";
  const mockLBHostedZoneID = "mockLBHostedZoneID";
  const mockResponseURL = "https://mock.com/";
  const mockRootDNSRole = "mockRootDNSRole";

  // Mock respond request.
  function mockFailedRequest(expectedErrMessageRegex) {
    return nock(mockResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED" && body.Reason.search(expectedErrMessageRegex) !== -1;
      })
      .reply(200);
  }

  let handler, reset, withDeadlineExpired;
  beforeEach(() => {
    // Prevent logging.
    console.log = function () {};

    // Reimport handlers so that the lazy loading does not fail the mocks.
    // A description of the issue can be found here: https://github.com/dwyl/aws-sdk-mock/issues/206.
    // This workaround follows the comment here: https://github.com/dwyl/aws-sdk-mock/issues/206#issuecomment-640418772.
    jest.resetModules();
    AWS.setSDKInstance(require("aws-sdk"));
    const imported = require("../lib/wkld-custom-domain");
    handler = imported.handler;
    reset = imported.reset;
    withDeadlineExpired = imported.withDeadlineExpired;

    // Mocks wait functions.
    imported.withSleep((_) => {
      return Promise.resolve();
    });
    withDeadlineExpired((_) => {
      return new Promise(function (resolve, reject) {});
    });
  });

  afterEach(() => {
    // Restore logger
    console.log = origLog;
    AWS.restore();
    reset();
  });

  describe("During CREATE with alias", () => {
    const mockRequest = {
      ResponseURL: mockResponseURL,
      ResourceProperties: {
        ServiceName: mockServiceName,
        Aliases: ["dash-test.mockDomain.com", "a.mockApp.mockDomain.com", "b.mockEnv.mockApp.mockDomain.com"],
        EnvName: mockEnvName,
        AppName: mockAppName,
        DomainName: mockDomainName,
        PublicAccessDNS: mockLBDNS,
        PublicAccessHostedZoneID: mockLBHostedZoneID,
        EnvHostedZoneId: mockEnvHostedZoneID,
        RootDNSRole: mockRootDNSRole,
      },
      RequestType: "Create",
      LogicalResourceId: "mockID",
    };

    // API call mocks.
    const mockListHostedZonesByName = sinon.stub();
    const mockListResourceRecordSets = sinon.stub();
    const mockChangeResourceRecordSets = sinon.stub();
    const mockWaitForRecordsChange = sinon.stub();
    const mockAppHostedZoneID = "mockAppHostedZoneID";
    const mockRootHostedZoneID = "mockRootHostedZoneID";

    beforeEach(() => {
      // Mock API default behavior.
      mockListResourceRecordSets.resolves({
        ResourceRecordSets: [],
      });
      mockChangeResourceRecordSets.resolves({
        ChangeInfo: { Id: "mockChangeID" },
      });
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockApp.mockDomain.com")).resolves({
        HostedZones: [
          {
            Id: mockAppHostedZoneID,
          },
        ],
      });
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockDomain.com")).resolves({
        HostedZones: [
          {
            Id: mockRootHostedZoneID,
          },
        ],
      });
      mockWaitForRecordsChange.resolves();
    });

    afterEach(() => {
      // Reset mocks call count.
      mockListHostedZonesByName.reset();
      mockListResourceRecordSets.reset();
      mockChangeResourceRecordSets.reset();
      mockWaitForRecordsChange.reset();
    });

    test("unsupported action fails", () => {
      let request = mockFailedRequest(/^Unsupported request type Unknown \(Log: .*\)$/);
      return LambdaTester(handler)
        .event({
          ResponseURL: mockResponseURL,
          ResourceProperties: {},
          RequestType: "Unknown",
          LogicalResourceId: "mockID",
        })
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
        });
    });

    test("error if an alias is not valid", () => {
      let request = mockFailedRequest(/^unrecognized domain type for Wow-this-domain-is-so-weird-that-it-does-not-work-at-all \(Log: .*\)$/);
      return LambdaTester(handler)
        .event({
          ResponseURL: mockResponseURL,
          ResourceProperties: {
            Aliases: ["Wow-this-domain-is-so-weird-that-it-does-not-work-at-all"],
          },
          RequestType: "Create",
        })
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
        });
    });

    test("error fetching app-level hosted zone ID", () => {
      const mockListHostedZonesByName = sinon.stub();
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockApp.mockDomain.com")).rejects(new Error("some error"));
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockDomain.com")).resolves({
        HostedZones: [
          {
            Id: mockRootHostedZoneID,
          },
        ],
      });
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2);
        });
    });

    test("error fetching root-level hosted zone ID", () => {
      const mockListHostedZonesByName = sinon.stub();
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockApp.mockDomain.com")).resolves({
        HostedZones: [
          {
            Id: mockAppHostedZoneID,
          },
        ],
      });
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockDomain.com")).rejects(new Error("some error"));
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
        });
    });

    test("error validating aliases", () => {
      const mockListResourceRecordSets = sinon.fake.rejects(new Error("some error"));
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListResourceRecordSets, 3); // 1 call for each alias; 3 aliases in total.
        });
    });

    test("some aliases are in use by other service", () => {
      const mockListResourceRecordSets = sinon.fake.resolves({
        ResourceRecordSets: [
          {
            AliasTarget: {
              DNSName: "other-lb-DNS",
            },
            Name: "dash-test.mockDomain.com.",
            Type: "A",
          },
        ],
      });
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);

      let request = mockFailedRequest(
        /^Alias dash-test.mockDomain.com is already in use by other-lb-DNS. This could be another load balancer of a different service. \(Log: .*\)$/
      );
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 call for each alias that is not env-level; there are 2 such aliases.
          sinon.assert.callCount(mockListResourceRecordSets, 3); // 1 call for each alias; 3 aliases in total.
        });
    });

    test("fail to upsert A-record for an alias into hosted zone", () => {
      const mockChangeResourceRecordSets = sinon.stub();
      mockChangeResourceRecordSets
        .withArgs(sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "dash-test.mockDomain.com"))
        .rejects(new Error("some error"));
      mockChangeResourceRecordSets.resolves({ ChangeInfo: { Id: "mockID" } });

      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 call for each alias that is not env-level; there are 2 such aliases.
          sinon.assert.callCount(mockListResourceRecordSets, 3); // 1 call for each alias; 3 aliases in total.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call for each alias; 3 aliases in total.
          sinon.assert.alwaysCalledWithMatch(mockChangeResourceRecordSets, sinon.match.hasNested("ChangeBatch.Changes[0].Action", "UPSERT"));
        });
    });

    test("fail to wait for resource record sets change to be finished", () => {
      const mockWaitFor = sinon.fake.rejects(new Error("some error"));
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitFor);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 call for each alias that is not env-level; there are 2 such aliases.
          sinon.assert.callCount(mockListResourceRecordSets, 3); // 1 call for each alias; 3 aliases in total.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call for each alias; 3 aliases in total.
          sinon.assert.callCount(mockWaitFor, 3); // 1 call for each alias; 3 aliases in total.
        });
    });

    test("lambda time out", () => {
      withDeadlineExpired((_) => {
        return new Promise(function (_, reject) {
          reject(new Error("lambda time out error"));
        });
      });
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", sinon.fake.resolves());
      AWS.mock("ACM", "waitFor", sinon.fake.resolves());

      let request = mockFailedRequest(/^lambda time out error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
        });
    });

    test("successful operation", () => {
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);

      // let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
        })
        .reply(200);

      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 call for each alias that is not env-level; there are 2 such aliases.
          sinon.assert.callCount(mockListResourceRecordSets, 3); // 1 call for each alias; 3 aliases in total.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call for each alias; 3 aliases in total.
          sinon.assert.callCount(mockWaitForRecordsChange, 3); // 1 call for each alias; 3 aliases in total.
          sinon.assert.alwaysCalledWithMatch(mockChangeResourceRecordSets, sinon.match.hasNested("ChangeBatch.Changes[0].Action", "UPSERT"));
        });
    });
  });

  describe("During DELETE", () => {
    const mockRequest = {
      ResponseURL: mockResponseURL,
      ResourceProperties: {
        ServiceName: mockServiceName,
        Aliases: ["a.mockDomain.com", "b.mockApp.mockDomain.com", "c.mockEnv.mockApp.mockDomain.com"],
        EnvName: mockEnvName,
        AppName: mockAppName,
        DomainName: mockDomainName,
        PublicAccessDNS: mockLBDNS,
        PublicAccessHostedZoneID: mockLBHostedZoneID,
        EnvHostedZoneId: mockEnvHostedZoneID,
        RootDNSRole: mockRootDNSRole,
      },
      RequestType: "Delete",
      LogicalResourceId: "mockID",
      PhysicalResourceId: "arn:mockARNToDelete",
    };

    // API call mocks.
    const mockListHostedZonesByName = sinon.stub();
    const mockChangeResourceRecordSets = sinon.stub();
    const mockWaitForRecordsChange = sinon.stub();
    const mockAppHostedZoneID = "mockAppHostedZoneID";
    const mockRootHostedZoneID = "mockRootHostedZoneID";
    beforeEach(() => {
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockApp.mockDomain.com")).resolves({
        HostedZones: [
          {
            Id: mockAppHostedZoneID,
          },
        ],
      });
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockDomain.com")).resolves({
        HostedZones: [
          {
            Id: mockRootHostedZoneID,
          },
        ],
      });
      mockChangeResourceRecordSets.withArgs(sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "a.mockDomain.com")).resolves({
        ChangeInfo: { Id: "mockID" },
      });
      mockChangeResourceRecordSets.withArgs(sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "b.mockApp.mockDomain.com")).resolves({
        ChangeInfo: { Id: "mockID" },
      });
      mockChangeResourceRecordSets
        .withArgs(sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "c.mockEnv.mockApp.mockDomain.com"))
        .resolves({
          ChangeInfo: { Id: "mockID" },
        });
      mockWaitForRecordsChange.withArgs("resourceRecordSetsChanged", sinon.match.has("Id", "mockID")).resolves();
    });

    afterEach(() => {
      // Reset mocks call count.
      mockListHostedZonesByName.reset();
      mockChangeResourceRecordSets.reset();
      mockWaitForRecordsChange.reset();
    });

    test("error removing A-record for an alias into hosted zone", () => {
      const mockChangeResourceRecordSets = sinon.stub();
      mockChangeResourceRecordSets
        .withArgs(sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "a.mockDomain.com"))
        .rejects(new Error("some error"));
      mockChangeResourceRecordSets.resolves({ ChangeInfo: { Id: "mockID" } });
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      let request = mockFailedRequest(/^delete record a.mockDomain.com: some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 call for each non-environment-level alias; there are 2 such aliases.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call for each alias; there are 3 aliases.
          sinon.assert.alwaysCalledWithMatch(mockChangeResourceRecordSets, sinon.match.hasNested("ChangeBatch.Changes[0].Action", "DELETE"));
        });
    });

    test("error waiting for resource record sets change to be finished", () => {
      const mockWaitForRecordsChange = sinon.fake.rejects(new Error("some error"));
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 call for each non-environment-level alias; there are 2 such aliases.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call for each alias; there are 3 aliases.
          sinon.assert.callCount(mockWaitForRecordsChange, 3); // 1 call for each alias; there are 3 aliases.
        });
    });

    test("do not error out if an A-record is not found", () => {
      const mockChangeResourceRecordSets = sinon.fake.rejects(
        new Error("Tried to delete resource record set [name='A.mockDomain.com', type='A'] but it was not found")
      );
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);

      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS";
        })
        .reply(200);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 call for each non-environment-level alias; there are 2 such aliases.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call for each alias; there are 3 aliases.
          sinon.assert.callCount(mockWaitForRecordsChange, 0); // Exited early when changeResourceRecordSets returns the not found error.
        });
    });

    test("do not error out if an A-record's value doesn't match", () => {
      const mockChangeResourceRecordSets = sinon.fake.rejects(
        new Error("Tried to delete resource record set [name='A.mockDomain.com', type='A'] but but the values provided do not match the current values")
      );
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);

      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS";
        })
        .reply(200);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 call for each non-environment-level alias; there are 2 such aliases.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call for each alias; there are 3 aliases.
          sinon.assert.callCount(mockWaitForRecordsChange, 0); // Exited early when changeResourceRecordSets returns the not found error.
        });
    });

    test("lambda time out", () => {
      withDeadlineExpired((_) => {
        return new Promise(function (_, reject) {
          reject(new Error("lambda time out error"));
        });
      });
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);

      let request = mockFailedRequest(/^lambda time out error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
        });
    });

    test("successful operation", () => {
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);

      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
        })
        .reply(200);

      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 call for each alias that is not env-level; there are 2 such aliases.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call for each alias; 3 aliases in total.
          sinon.assert.callCount(mockWaitForRecordsChange, 3); // 1 call for each alias; 3 aliases in total.
          sinon.assert.alwaysCalledWithMatch(mockChangeResourceRecordSets, sinon.match.hasNested("ChangeBatch.Changes[0].Action", "DELETE"));
        });
    });
  });

  describe("During UPDATE", () => {
    let mockRequest;

    // API call mocks.
    const mockListHostedZonesByName = sinon.stub();
    const mockListResourceRecordSets = sinon.stub();
    const mockChangeResourceRecordSets = sinon.stub();
    const mockWaitForRecordsChange = sinon.stub();
    const mockAppHostedZoneID = "mockAppHostedZoneID";
    const mockRootHostedZoneID = "mockRootHostedZoneID";

    beforeEach(() => {
      // Reset mockRequest.
      mockRequest = {
        ResponseURL: mockResponseURL,
        ResourceProperties: {
          ServiceName: mockServiceName,
          EnvName: mockEnvName,
          AppName: mockAppName,
          DomainName: mockDomainName,
          PublicAccessDNS: mockLBDNS,
          PublicAccessHostedZoneID: mockLBHostedZoneID,
          EnvHostedZoneId: mockEnvHostedZoneID,
          RootDNSRole: mockRootDNSRole,
        },
        OldResourceProperties: {
          ServiceName: mockServiceName,
          EnvName: mockEnvName,
          AppName: mockAppName,
          DomainName: mockDomainName,
          PublicAccessDNS: mockLBDNS,
          PublicAccessHostedZoneID: mockLBHostedZoneID,
          EnvHostedZoneId: mockEnvHostedZoneID,
          RootDNSRole: mockRootDNSRole,
        },
        RequestType: "Update",
        LogicalResourceId: "mockID",
      };

      // Mock API default behavior.
      mockListResourceRecordSets.resolves({
        ResourceRecordSets: [],
      });
      mockChangeResourceRecordSets.resolves({
        ChangeInfo: { Id: "mockChangeID" },
      });
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockApp.mockDomain.com")).resolves({
        HostedZones: [
          {
            Id: mockAppHostedZoneID,
          },
        ],
      });
      mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockDomain.com")).resolves({
        HostedZones: [
          {
            Id: mockRootHostedZoneID,
          },
        ],
      });
      mockWaitForRecordsChange.resolves();
    });

    afterEach(() => {
      // Reset mocks call count.
      mockListHostedZonesByName.reset();
      mockListResourceRecordSets.reset();
      mockChangeResourceRecordSets.reset();
      mockWaitForRecordsChange.reset();
    });

    test("do nothing if the new aliases are exactly the same as the old one", () => {
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);

      mockRequest.ResourceProperties.Aliases = ["a.mockDomain.com", "b.mockDomain.com", "b.mockDomain.com"];
      mockRequest.OldResourceProperties.Aliases = ["b.mockDomain.com", "a.mockDomain.com", "a.mockDomain.com"];
      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
        })
        .reply(200);

      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          // Called nothing.
          sinon.assert.callCount(mockListResourceRecordSets, 0);
          sinon.assert.callCount(mockListHostedZonesByName, 0);
          sinon.assert.callCount(mockChangeResourceRecordSets, 0);
          sinon.assert.callCount(mockWaitForRecordsChange, 0);
        });
    });

    test("update if the new aliases are exactly the same as the old ones but hosted zone and dns change", () => {
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets); // Calls to validate aliases.
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets); // Calls to upsert the A-records.
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange); // Calls to wait for the changes.

      mockRequest.ResourceProperties.Aliases = ["a.mockDomain.com"];
      mockRequest.OldResourceProperties.Aliases = ["a.mockDomain.com"];

      mockRequest.ResourceProperties.PublicAccessHostedZoneID = "mockNewHostedZoneID"
      mockRequest.OldResourceProperties.PublicAccessHostedZoneID = "mockHostedZoneID"

      mockRequest.ResourceProperties.PublicAccessDNS = "mockNewDNS"
      mockRequest.OldResourceProperties.PublicAccessDNS = "mockDNS"

      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
        })
        .reply(200);

        return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 1); 
          sinon.assert.callCount(mockChangeResourceRecordSets, 1); 
          sinon.assert.callCount(mockWaitForRecordsChange, 1);

          // The following calls are made to add aliases.
          sinon.assert.callCount(mockListResourceRecordSets, 1); // 1 call to each alias to validate its ownership; there is 1 alias.
          sinon.assert.calledWithMatch(mockChangeResourceRecordSets.getCall(0), sinon.match.hasNested("ChangeBatch.Changes[0].Action", "UPSERT"));
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "a.mockDomain.com")
          );
        });
    });

    test("update if the aliases, hosted zone, and dns change", () => {
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets); // Calls to validate aliases.
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets); // Calls to upsert the A-records.
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange); // Calls to wait for the changes.

      mockRequest.ResourceProperties.Aliases = ["a.mockDomain.com"];
      mockRequest.OldResourceProperties.Aliases = ["b.mockDomain.com"];

      mockRequest.ResourceProperties.PublicAccessHostedZoneID = "mockNewHostedZoneID"
      mockRequest.OldResourceProperties.PublicAccessHostedZoneID = "mockHostedZoneID"

      mockRequest.ResourceProperties.PublicAccessDNS = "mockNewDNS"
      mockRequest.OldResourceProperties.PublicAccessDNS = "mockDNS"

      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
        })
        .reply(200);

        return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 1); 
          sinon.assert.callCount(mockChangeResourceRecordSets, 2); 
          sinon.assert.callCount(mockWaitForRecordsChange, 2);

          // The following calls are made to add aliases.
          sinon.assert.callCount(mockListResourceRecordSets, 1); // 1 call to each alias to validate its ownership; there is 1 alias.
          sinon.assert.calledWithMatch(mockChangeResourceRecordSets.getCall(0), sinon.match.hasNested("ChangeBatch.Changes[0].Action", "UPSERT"));
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "b.mockDomain.com")
          );

          // The following calls are made to remove aliases.
          sinon.assert.calledWithMatch(mockChangeResourceRecordSets, sinon.match.hasNested("ChangeBatch.Changes[0].Action", "DELETE"));
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "a.mockDomain.com")
          );
        });
    });

    test("new aliases that only add additional aliases to the old aliases, without deletion", () => {
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets); // Calls to validate aliases.
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets); // Calls to upsert the A-records.
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange); // Calls to wait for the changes.

      mockRequest.ResourceProperties.Aliases = ["a.mockDomain.com", "b.mockApp.mockDomain.com", "c.mockEnv.mockApp.mockDomain.com"];
      mockRequest.OldResourceProperties.Aliases = ["a.mockDomain.com"];
      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
        })
        .reply(200);

      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 calls to each non-env-level aliases; there are 2 such aliases.
          sinon.assert.callCount(mockListResourceRecordSets, 3); // 1 call to each alias to validate its ownership; there are 3 aliases.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call to each alias to upsert its A-record; there are 3 aliases.
          sinon.assert.callCount(mockWaitForRecordsChange, 3); // 1 call to each alias after upserting A-record; there are 3 aliases.
        });
    });

    test("new aliases that only delete some aliases from the old aliases, without addition", () => {
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets); // Calls to validate aliases.
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets); // Calls to upsert the A-records.
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange); // Calls to wait for the changes.

      mockRequest.ResourceProperties.Aliases = ["a.mockDomain.com"];
      mockRequest.OldResourceProperties.Aliases = ["a.mockDomain.com", "b.mockApp.mockDomain.com", "c.mockEnv.mockApp.mockDomain.com"];
      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
        })
        .reply(200);

      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 calls to each non-env-level aliases; there are 2 such aliases.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 1 call to upsert the alias, 2 calls to remove unused aliases.
          sinon.assert.callCount(mockWaitForRecordsChange, 3); // 1 call following each `ChangeResourceRecordSets`.

          // The following calls are made to add aliases.
          // Although the aliases already exist (in `OldResourceProperties`), we repeat these operations anyway just to be sure.
          sinon.assert.callCount(mockListResourceRecordSets, 1); // 1 call to each alias to validate its ownership; there are 1 alias.
          sinon.assert.calledWithMatch(mockChangeResourceRecordSets.getCall(0), sinon.match.hasNested("ChangeBatch.Changes[0].Action", "UPSERT"));
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets.getCall(0),
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "a.mockDomain.com")
          );

          // The following calls are made to remove aliases.
          sinon.assert.calledWithMatch(mockChangeResourceRecordSets.getCall(1), sinon.match.hasNested("ChangeBatch.Changes[0].Action", "DELETE"));
          sinon.assert.calledWithMatch(mockChangeResourceRecordSets.getCall(2), sinon.match.hasNested("ChangeBatch.Changes[0].Action", "DELETE"));
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "b.mockApp.mockDomain.com")
          );
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "c.mockEnv.mockApp.mockDomain.com")
          );
        });
    });

    test("new aliases that both add to and remove from the aliases", () => {
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets); // Calls to validate aliases.
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets); // Calls to upsert the A-records.
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange); // Calls to wait for the changes.

      mockRequest.ResourceProperties.Aliases = ["has-always-been.mockApp.mockDomain.com", "new.mockEnv.mockApp.mockDomain.com"];
      mockRequest.OldResourceProperties.Aliases = ["has-always-been.mockApp.mockDomain.com", "unused.mockDomain.com"];
      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockID";
        })
        .reply(200);

      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2); // 1 calls to each non-env-level aliases; there are 2 such aliases.
          sinon.assert.callCount(mockChangeResourceRecordSets, 3); // 2 call to upsert the alias, 1 calls to remove unused aliases.
          sinon.assert.callCount(mockWaitForRecordsChange, 3); // 1 call following each `ChangeResourceRecordSets`.

          // The following calls are made to add aliases.
          sinon.assert.callCount(mockListResourceRecordSets, 2); // 1 call to each alias to validate its ownership; there are 2 alias.
          sinon.assert.calledWithMatch(mockChangeResourceRecordSets.getCall(0), sinon.match.hasNested("ChangeBatch.Changes[0].Action", "UPSERT"));
          sinon.assert.calledWithMatch(mockChangeResourceRecordSets.getCall(1), sinon.match.hasNested("ChangeBatch.Changes[0].Action", "UPSERT"));
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "has-always-been.mockApp.mockDomain.com")
          );
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "new.mockEnv.mockApp.mockDomain.com")
          );

          // The following calls are made to remove aliases.
          sinon.assert.calledWithMatch(mockChangeResourceRecordSets.getCall(2), sinon.match.hasNested("ChangeBatch.Changes[0].Action", "DELETE"));
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets.getCall(2),
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "unused.mockDomain.com")
          );
        });
    });
  });
});
