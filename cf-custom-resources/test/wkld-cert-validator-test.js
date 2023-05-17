// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const AWS = require("aws-sdk-mock");
const LambdaTester = require("lambda-tester").noVersionCheck();
const sinon = require("sinon");
const nock = require("nock");
let origLog = console.log;

const { attemptsValidationOptionsReady } = require("../lib/wkld-cert-validator");

describe("DNS Certificate Validation And Custom Domains", () => {
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
    const imported = require("../lib/wkld-cert-validator");
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
        LoadBalancerDNS: mockLBDNS,
        LoadBalancerHostedZoneID: mockLBHostedZoneID,
        EnvHostedZoneId: mockEnvHostedZoneID,
        RootDNSRole: mockRootDNSRole,
      },
      RequestType: "Create",
      LogicalResourceId: "mockID",
    };

    // API call mocks.
    const mockListHostedZonesByName = sinon.stub();
    const mockListResourceRecordSets = sinon.stub();
    const mockRequestCertificate = sinon.stub();
    const mockDescribeCertificate = sinon.stub();
    const mockChangeResourceRecordSets = sinon.stub();
    const mockWaitForRecordsChange = sinon.stub();
    const mockWaitForCertificateValidation = sinon.stub();
    const mockAppHostedZoneID = "mockAppHostedZoneID";
    const mockRootHostedZoneID = "mockRootHostedZoneID";

    beforeEach(() => {
      // Mock API default behavior.
      mockListResourceRecordSets.resolves({
        ResourceRecordSets: [],
      });
      mockRequestCertificate.resolves({
        CertificateArn: "mockCertArn",
      });
      mockDescribeCertificate.resolves({
        Certificate: {
          DomainValidationOptions: [
            {
              ResourceRecord: {
                Name: "mock-validate-default-cert",
                Value: "mock-validate-default-cert-value",
                Type: "mock-validate-default-cert-type",
              },
              DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
            },
            {
              ResourceRecord: {
                Name: "mock-validate-alias-1",
                Value: "mock-validate-alias-1-value",
                Type: "mock-validate-alias-1-type",
              },
              DomainName: "dash-test.mockDomain.com",
            },
            {
              ResourceRecord: {
                Name: "mock-validate-alias-2",
                Value: "mock-validate-alias-2-value",
                Type: "mock-validate-alias-2-type",
              },
              DomainName: "a.mockApp.mockDomain.com",
            },
            {
              ResourceRecord: {
                Name: "mock-validate-alias-3",
                Value: "mock-validate-alias-3-value",
                Type: "mock-validate-alias-3-type",
              },
              DomainName: "b.mockEnv.mockApp.mockDomain.com",
            },
          ],
        },
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
      mockWaitForCertificateValidation.resolves();
    });

    afterEach(() => {
      // Reset mocks call count.
      mockListHostedZonesByName.reset();
      mockListResourceRecordSets.reset();
      mockRequestCertificate.reset();
      mockDescribeCertificate.reset();
      mockChangeResourceRecordSets.reset();
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
          sinon.assert.callCount(mockListResourceRecordSets, 3);
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
          sinon.assert.callCount(mockListHostedZonesByName, 2);
          sinon.assert.callCount(mockListResourceRecordSets, 3);
        });
    });

    test("fail to request a certificate", () => {
      const mockRequestCertificate = sinon.fake.rejects(new Error("some error"));
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("ACM", "requestCertificate", mockRequestCertificate);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2);
          sinon.assert.callCount(mockListResourceRecordSets, 3);
          sinon.assert.callCount(mockRequestCertificate, 1);
        });
    });

    test("timed out waiting for validation options to be ready", () => {
      const mockDescribeCertificate = sinon.fake.resolves({
        Certificate: {
          DomainValidationOptions: [
            {
              ResourceRecord: {},
              DomainName: "not the domain we want",
            },
          ],
        },
      });
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);

      let request = mockFailedRequest(/^resource validation records are not ready after 10 tries \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2);
          sinon.assert.callCount(mockListResourceRecordSets, 3);
          sinon.assert.callCount(mockRequestCertificate, 1);
          sinon.assert.callCount(mockDescribeCertificate, attemptsValidationOptionsReady);
        });
    });

    test("error while waiting for validation options to be ready", () => {
      const mockDescribeCertificate = sinon.fake.rejects(new Error("some error"));
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2);
          sinon.assert.callCount(mockListResourceRecordSets, 3);
          sinon.assert.callCount(mockRequestCertificate, 1);
          sinon.assert.callCount(mockDescribeCertificate, 1);
        });
    });

    test("fail to upsert validation record for an alias into hosted zone", () => {
      const mockChangeResourceRecordSets = sinon.stub();
      mockChangeResourceRecordSets.rejects(new Error("some error"));

      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2);
          sinon.assert.callCount(mockListResourceRecordSets, 3);
          sinon.assert.callCount(mockRequestCertificate, 1);
          sinon.assert.callCount(mockDescribeCertificate, 1);
          sinon.assert.callCount(mockChangeResourceRecordSets, 4);
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "mock-validate-default-cert")
          );
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "mock-validate-alias-1")
          );
        });
    });

    test("fail to wait for resource record sets change to be finished", () => {
      const mockWaitFor = sinon.fake.rejects(new Error("some error"));
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitFor);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2);
          sinon.assert.callCount(mockListResourceRecordSets, 3);
          sinon.assert.callCount(mockRequestCertificate, 1);
          sinon.assert.callCount(mockDescribeCertificate, 1);
          sinon.assert.callCount(mockChangeResourceRecordSets, 4);
          sinon.assert.callCount(mockWaitFor, 4);
        });
    });

    test("fail to wait for certificate to be validated", () => {
      const mockWaitForRecordsChange = sinon.stub();
      mockWaitForRecordsChange.withArgs("resourceRecordSetsChanged", sinon.match.has("Id", "mockChangeID")).resolves();
      const mockWaitForCertificateValidation = sinon.stub();
      mockWaitForCertificateValidation.withArgs("certificateValidated", sinon.match.has("CertificateArn", "mockCertArn")).rejects(new Error("some error"));

      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);
      AWS.mock("ACM", "waitFor", mockWaitForCertificateValidation);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2);
          sinon.assert.callCount(mockListResourceRecordSets, 3);
          sinon.assert.callCount(mockRequestCertificate, 1);
          sinon.assert.callCount(mockDescribeCertificate, 1);
          sinon.assert.callCount(mockChangeResourceRecordSets, 4);
          sinon.assert.callCount(mockWaitForRecordsChange, 4);
          sinon.assert.callCount(mockWaitForCertificateValidation, 1);
        });
    });

    test("lambda time out", () => {
      withDeadlineExpired((_) => {
        return new Promise(function (_, reject) {
          reject(new Error("lambda time out error"));
        });
      });
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
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
      AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);
      AWS.mock("ACM", "waitFor", mockWaitForCertificateValidation);

      // let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      let request = nock(mockResponseURL)
        .put("/", (body) => {
          return body.Status === "SUCCESS" && body.PhysicalResourceId === "mockCertArn";
        })
        .reply(200);

      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockListHostedZonesByName, 2);
          sinon.assert.callCount(mockListResourceRecordSets, 3);
          sinon.assert.callCount(mockRequestCertificate, 1);
          sinon.assert.callCount(mockDescribeCertificate, 1);
          sinon.assert.callCount(mockChangeResourceRecordSets, 4);
          sinon.assert.callCount(mockWaitForRecordsChange, 4);
          sinon.assert.callCount(mockWaitForCertificateValidation, 1);
        });
    });
  });

  describe("During DELETE", () => {
    const mockRequest = {
      ResponseURL: mockResponseURL,
      ResourceProperties: {
        ServiceName: mockServiceName,
        Aliases: [
          "unused.mockDomain.com",
          "usedByNewCert.mockApp.mockDomain.com",
          "usedByOtherCert.mockApp.mockDomain.com",
          "usedByOtherService.mockEnv.mockApp.mockDomain.com",
        ],
        EnvName: mockEnvName,
        AppName: mockAppName,
        DomainName: mockDomainName,
        LoadBalancerDNS: mockLBDNS,
        LoadBalancerHostedZoneID: mockLBHostedZoneID,
        EnvHostedZoneId: mockEnvHostedZoneID,
        RootDNSRole: mockRootDNSRole,
      },
      RequestType: "Delete",
      LogicalResourceId: "mockID",
      PhysicalResourceId: "arn:mockARNToDelete",
    };

    // API call mocks.
    const mockListHostedZonesByName = sinon.stub();
    const mockGetResources = sinon.stub();
    const mockDescribeCertificate = sinon.stub();
    const mockListResourceRecordSets = sinon.stub();
    const mockDeleteCertificate = sinon.stub();
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
      mockGetResources.resolves({
        ResourceTagMappingList: [{ ResourceARN: "arn:mockARNToDelete" }, { ResourceARN: "arn:mockARNInUse" }, { ResourceARN: "arn:mockARNInUse2" }],
      });

      mockDescribeCertificate
        .withArgs(sinon.match({ CertificateArn: "arn:mockARNToDelete" }))
        .onFirstCall()
        .resolves({
          Certificate: {
            CertificateArn: "arn:mockARNToDelete",
            DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
            SubjectAlternativeNames: [
              "usedByOtherService.mockEnv.mockApp.mockDomain.com",
              "usedByOtherCert.mockApp.mockDomain.com",
              "unused.mockDomain.com",
              "usedByNewCert.mockApp.mockDomain.com",
              `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
            ],
            DomainValidationOptions: [
              {
                DomainName: "unused.mockDomain.com",
                ResourceRecord: {
                  Name: "validate.unused.mockDomain.com",
                  Type: "CNAME",
                  Value: "validate.unused.mockDomain.com.v",
                },
              },
              {
                DomainName: "usedByNewCert.mockApp.mockDomain.com",
                ResourceRecord: {
                  Name: "validate.usedByNewCert.mockApp.mockDomain.com",
                  Type: "CNAME",
                  Value: "validate.usedByNewCert.mockApp.mockDomain.com.v",
                },
              },
              {
                DomainName: "usedByOtherService.mockEnv.mockApp.mockDomain.com",
                ResourceRecord: {
                  Name: "validate.usedByOtherService.mockEnv.mockApp.mockDomain.com",
                  Type: "CNAME",
                  Value: "validate.usedByOtherService.mockEnv.mockApp.mockDomain.com.v",
                },
              },
              {
                DomainName: "usedByOtherCert.mockApp.mockDomain.com",
                ResourceRecord: {
                  Name: "validate.usedByOtherCert.mockApp.mockDomain.com",
                  Type: "CNAME",
                  Value: "validate.usedByOtherCert.mockApp.mockDomain.com.v",
                },
              },
              {
                DomainName: "random.unrecognized.domain",
                ResourceRecord: {
                  Name: "validate.random.unrecognized.domain",
                  Type: "CNAME",
                  Value: "validate.random.unrecognized.domain.v",
                },
              },
              {
                DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
              },
            ],
          },
        });
      mockDescribeCertificate.withArgs(sinon.match({ CertificateArn: "arn:mockARNInUse" })).resolves({
        Certificate: {
          CertificateArn: "mockARNInUse",
          DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
          SubjectAlternativeNames: [
            "usedByNewCert.mockApp.mockDomain.com",
            "other.mockDomain.com",
            `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
          ],
          DomainValidationOptions: [
            {
              DomainName: "usedByNewCert.mockApp.mockDomain.com",
              ResourceRecord: {
                Name: "validate.usedByNewCert.mockApp.mockDomain.com",
                Type: "CNAME",
                Value: "validate.usedByNewCert.mockApp.mockDomain.com.v",
              },
            },
            {
              DomainName: "other.mockDomain.com",
              ResourceRecord: {
                Name: "validate.other.mockDomain.com",
                Type: "CNAME",
                Value: "validate.other.mockDomain.com.v",
              },
            },
            {
              DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
              ResourceRecord: {
                Name: "validate.canonical.default.cert.domain",
                Type: "CNAME",
                Value: "validate.canonical.default.cert.domain.v",
              },
            },
          ],
        },
      });
      mockDescribeCertificate.withArgs(sinon.match({ CertificateArn: "arn:mockARNInUse2" })).resolves({
        Certificate: {
          CertificateArn: "mockARNInUse2",
          DomainName: `some.domain.name.that.is.not.default.hence.not.created.by.copilot`,
          SubjectAlternativeNames: [
            "usedByOtherCert.mockApp.mockDomain.com",
            "other.mockDomain.com",
            `some.domain.name.that.is.not.default.hence.not.created.by.copilot`,
          ],
          DomainValidationOptions: [
            {
              DomainName: "usedByOtherCert.mockApp.mockDomain.com",
              ResourceRecord: {
                Name: "validate.usedByOtherCert.mockApp.mockDomain.com",
                Type: "CNAME",
                Value: "validate.usedByOtherCert.mockApp.mockDomain.com.v",
              },
            },
            {
              DomainName: "other.mockDomain.com",
              ResourceRecord: {
                Name: "validate.other.mockDomain.com",
                Type: "CNAME",
                Value: "validate.other.mockDomain.com.v",
              },
            },
            {
              DomainName: "some.domain.name.that.is.not.default.hence.not.created.by.copilot",
              ResourceRecord: {
                Name: "validate.some.domain.name.that.is.not.default.hence.not.created.by.copilot",
                Type: "CNAME",
                Value: "validate.some.domain.name.that.is.not.default.hence.not.created.by.copilot.v",
              },
            },
          ],
        },
      });
      mockListResourceRecordSets
        .withArgs(
          sinon.match({
            HostedZoneId: "mockRootHostedZoneID",
            StartRecordName: "unused.mockDomain.com",
          })
        )
        .resolves({
          ResourceRecordSets: [
            {
              Name: "unused.mockDomain.com.",
              AliasTarget: {
                DNSName: `${mockLBDNS}.`,
              },
            },
          ],
        });
      mockListResourceRecordSets
        .withArgs(
          sinon.match({
            HostedZoneId: mockEnvHostedZoneID,
            StartRecordName: "usedByOtherService.mockEnv.mockApp.mockDomain.com",
          })
        )
        .resolves({
          ResourceRecordSets: [
            {
              Name: "usedByOtherService.mockEnv.mockApp.mockDomain.com.",
              AliasTarget: {
                DNSName: "other.service.dns.name.",
              },
            },
          ],
        });
      mockDeleteCertificate.withArgs({ CertificateArn: "arn:mockARNToDelete" }).resolves();
      mockChangeResourceRecordSets.withArgs(sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "validate.unused.mockDomain.com")).resolves({
        ChangeInfo: { Id: "mockID" },
      });
      mockWaitForRecordsChange.withArgs("resourceRecordSetsChanged", sinon.match.has("Id", "mockID")).resolves();
    });

    afterEach(() => {
      // Reset mocks call count.
      mockListHostedZonesByName.reset();
      mockGetResources.reset();
      mockDescribeCertificate.reset();
      mockListResourceRecordSets.reset();
      mockDeleteCertificate.reset();
      mockDeleteCertificate.reset();
    });

    test("error retrieving service certificate by tags", () => {
      const mockGetResources = sinon.stub().rejects(new Error("some error"));
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.calledWith(mockGetResources, {
            TagFilters: [
              {
                Key: "copilot-application",
                Values: [mockAppName],
              },
              {
                Key: "copilot-environment",
                Values: [mockEnvName],
              },
              {
                Key: "copilot-service",
                Values: [mockServiceName],
              },
            ],
            ResourceTypeFilters: ["acm:certificate"],
          });
        });
    });

    test("error describing certificate", () => {
      const mockDescribeCertificate = sinon.stub().rejects(new Error("some error"));
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockGetResources, 1);
          sinon.assert.callCount(mockDescribeCertificate, 3);
        });
    });

    test("error listing resource record set", () => {
      const mockListResourceRecordSets = sinon.stub().rejects(new Error("some error"));
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockGetResources, 1);
          sinon.assert.callCount(mockDescribeCertificate, 3);
        });
    });

    test("error removing validation record for an alias into hosted zone", () => {
      const mockChangeResourceRecordSets = sinon.stub();
      mockChangeResourceRecordSets
        .withArgs(sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "validate.unused.mockDomain.com"))
        .rejects(new Error("some error"));
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      let request = mockFailedRequest(/^delete record validate.unused.mockDomain.com: some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockGetResources, 1);
          sinon.assert.callCount(mockDescribeCertificate, 3);
          sinon.assert.callCount(mockChangeResourceRecordSets, 1); // Only one validation option is to be deleted.
        });
    });

    test("error waiting for resource record sets change to be finished", () => {
      const mockWaitForRecordsChange = sinon.fake.rejects(new Error("some error"));
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);

      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockGetResources, 1);
          sinon.assert.callCount(mockDescribeCertificate, 3);
          sinon.assert.callCount(mockChangeResourceRecordSets, 1); // Only one validation option is to be deleted.
          sinon.assert.callCount(mockWaitForRecordsChange, 1);
          sinon.assert.calledWithMatch(
            mockChangeResourceRecordSets,
            sinon.match.hasNested("ChangeBatch.Changes[0].ResourceRecordSet.Name", "validate.unused.mockDomain.com")
          );
        });
    });

    test("error waiting for certificate to be unused", () => {
      mockDescribeCertificate.resolves({
        Certificate: {
          InUseBy: ["inuse"],
        },
      });
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);

      let request = mockFailedRequest(/^Certificate still in use after checking for 12 attempts. \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockGetResources, 1);
          sinon.assert.callCount(mockDescribeCertificate, 15); // 3 call to list unused options, 12 calls to wait for the certificate to be unused.
        });
    });

    test("error deleting certificate", () => {
      const mockDeleteCertificate = sinon.stub().rejects(new Error("some error"));
      mockDescribeCertificate.resolves({
        Certificate: {
          InUseBy: [],
        },
      });
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
      AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);
      AWS.mock("ACM", "deleteCertificate", mockDeleteCertificate);
      let request = mockFailedRequest(/^some error \(Log: .*\)$/);
      return LambdaTester(handler)
        .event(mockRequest)
        .expectResolve(() => {
          expect(request.isDone()).toBe(true);
          sinon.assert.callCount(mockGetResources, 1);
          sinon.assert.callCount(mockDescribeCertificate, 4); // 3 call to list unused options, 1 calls to find out that the certificate is unused.
          sinon.assert.callCount(mockDeleteCertificate, 1);
        });
    });

    test("do not error out if a validation record is not found", () => {
      const mockChangeResourceRecordSets = sinon.fake.rejects(
        new Error("Tried to delete resource record set [name='validate.unused.mockDomain.com', type='CNAME'] but it was not found")
      );
      // Since we don't expect the above error to interrupt the process, we need to mock the actions following `ChangeResourceRecordSets` as well.
      let resourceNotFoundErr = new Error("some error");
      resourceNotFoundErr.name = "ResourceNotFoundException";
      mockDescribeCertificate.rejects(resourceNotFoundErr);
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
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
          sinon.assert.callCount(mockGetResources, 1);
          sinon.assert.callCount(mockDescribeCertificate, 4); // 3 call to list unused options, 1 calls to find out that the certificate is already not found.
          sinon.assert.callCount(mockChangeResourceRecordSets, 1); // Only one validation option is to be deleted.
        });
    });

    test("do not error out if certificate is not found while waiting for it to be unused", () => {
      let resourceNotFoundErr = new Error("some error");
      resourceNotFoundErr.name = "ResourceNotFoundException";
      mockDescribeCertificate.rejects(resourceNotFoundErr);
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("ACM", "deleteCertificate", mockDeleteCertificate);
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
          sinon.assert.callCount(mockGetResources, 1);
          sinon.assert.callCount(mockDescribeCertificate, 4); // 3 call to list unused options, 1 calls to find out that the certificate is already not found.
        });
    });

    test("do not error out if certificate is not found while deleting", () => {
      mockDescribeCertificate.resolves({
        Certificate: {
          InUseBy: [],
        },
      });
      let resourceNotFoundErr = new Error("some error");
      resourceNotFoundErr.name = "ResourceNotFoundException";
      const mockDeleteCertificate = sinon.stub().rejects(resourceNotFoundErr);
      AWS.mock("ResourceGroupsTaggingAPI", "getResources", mockGetResources);
      AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
      AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
      AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
      AWS.mock("ACM", "deleteCertificate", mockDeleteCertificate);
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
          sinon.assert.callCount(mockGetResources, 1);
          sinon.assert.callCount(mockDescribeCertificate, 4); // 3 call to list unused options, 1 calls to find out that the certificate is unused.
        });
    });
  });
});
