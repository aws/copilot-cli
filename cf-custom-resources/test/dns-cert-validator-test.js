// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

describe("DNS Validated Certificate Handler", () => {
  const AWS = require("aws-sdk-mock");
  const LambdaTester = require("lambda-tester").noVersionCheck();
  const sinon = require("sinon");
  const handler = require("../lib/dns-cert-validator");
  const nock = require("nock");
  const ResponseURL = "https://cloudwatch-response-mock.example.com/";

  let origLog = console.log;
  const testRequestId = "f4ef1b10-c39a-44e3-99c0-fbf7e53c3943";
  const testAppName = "myapp";
  const testEnvName = "test";
  const testDomainName = "example.com";
  const testHostedZoneId = "Z3P5QSUBK4POTI";
  const testAppHostedZoneId = "K4POTIZ3P5QSUB";
  const testRootDNSRole = "mockRole";
  const testSubjectAlternativeNames = [
    "example.com",
    "*.example.com",
    "myapp.example.com",
    "*.myapp.example.com",
    "*.test.myapp.example.com",
  ];
  const testCopilotTags = [
    { Key: "copilot-application", Value: testAppName },
    { Key: "copilot-environment", Value: testEnvName },
  ];
  const testCertificateArn =
    "arn:aws:acm:region:123456789012:certificate/12345678-1234-1234-1234-123456789012";
  const testRRName = "_3639ac514e785e898d2646601fa951d5.example.com";
  const testRRValue = "_x2.acm-validations.aws";
  const spySleep = sinon.spy(function (ms) {
    return Promise.resolve();
  });
  const testDeleteRecordChangebatch = {
    Changes: [
      {
        Action: "DELETE",
        ResourceRecordSet: {
          Name: testRRName,
          Type: "CNAME",
          TTL: 60,
          ResourceRecords: [
            {
              Value: testRRValue,
            },
          ],
        },
      },
    ],
  };
  const testUpsertRecordChangebatch = {
    Changes: [
      {
        Action: "UPSERT",
        ResourceRecordSet: {
          Name: testRRName,
          Type: "CNAME",
          TTL: 60,
          ResourceRecords: [
            {
              Value: testRRValue,
            },
          ],
        },
      },
    ],
  };
  const legacyCertValidatorOptions = [
    {
      DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
      ValidationStatus: "SUCCESS",
      ResourceRecord: {
        Name: testRRName,
        Type: "CNAME",
        Value: testRRValue,
      },
    },
  ];
  const newCertValidateOptions = [
    {
      DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
      ValidationStatus: "SUCCESS",
      ResourceRecord: {
        Name: testRRName,
        Type: "CNAME",
        Value: testRRValue,
      },
    },
    {
      DomainName: `${testAppName}.${testDomainName}`,
      ValidationStatus: "SUCCESS",
      ResourceRecord: {
        Name: testRRName,
        Type: "CNAME",
        Value: testRRValue,
      },
    },
    {
      DomainName: `${testDomainName}`,
      ValidationStatus: "SUCCESS",
      ResourceRecord: {
        Name: testRRName,
        Type: "CNAME",
        Value: testRRValue,
      },
    },
  ];

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
    handler.withSleep(spySleep);
    console.log = function () {};
  });
  afterEach(() => {
    // Restore waiters and logger
    handler.reset();
    AWS.restore();
    console.log = origLog;
    spySleep.resetHistory();
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
    return LambdaTester(handler.certificateRequestHandler)
      .event({})
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
    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: bogusType,
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation requests a new certificate", () => {
    const requestCertificateFake = sinon.fake.resolves({
      CertificateArn: testCertificateArn,
    });

    const describeCertificateFake = sinon.stub();
    describeCertificateFake.onFirstCall().resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
      },
    });
    describeCertificateFake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        DomainValidationOptions: newCertValidateOptions,
      },
    });

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

    AWS.mock("ACM", "requestCertificate", requestCertificateFake);
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);
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

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          IsAliasEnabled: "true",
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
          SubjectAlternativeNames: testSubjectAlternativeNames,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.calledWith(
          requestCertificateFake,
          sinon.match({
            DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
            SubjectAlternativeNames: testSubjectAlternativeNames,
            ValidationMethod: "DNS",
            Tags: testCopilotTags,
          })
        );
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: testUpsertRecordChangebatch,
            HostedZoneId: testHostedZoneId,
          })
        );
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: `${testAppName}.${testDomainName}`,
            MaxItems: "1",
          })
        );
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: `${testDomainName}`,
            MaxItems: "1",
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation requests a legacy certificate", () => {
    const requestCertificateFake = sinon.fake.resolves({
      CertificateArn: testCertificateArn,
    });

    const describeCertificateFake = sinon.stub();
    describeCertificateFake.onFirstCall().resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
      },
    });
    describeCertificateFake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        DomainValidationOptions: legacyCertValidatorOptions,
      },
    });

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    AWS.mock("ACM", "requestCertificate", requestCertificateFake);
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);
    AWS.mock(
      "Route53",
      "changeResourceRecordSets",
      changeResourceRecordSetsFake
    );

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          IsAliasEnabled: "false",
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
          SubjectAlternativeNames: testSubjectAlternativeNames,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.calledWith(
          requestCertificateFake,
          sinon.match({
            DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
            SubjectAlternativeNames: [
              `*.${testEnvName}.${testAppName}.${testDomainName}`,
            ],
            ValidationMethod: "DNS",
            Tags: testCopilotTags,
          })
        );
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: testUpsertRecordChangebatch,
            HostedZoneId: testHostedZoneId,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation fails after more than 60s if certificate has no DomainValidationOptions", () => {
    handler.withRandom(() => 0);
    const requestCertificateFake = sinon.fake.resolves({
      CertificateArn: testCertificateArn,
    });

    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
      },
      DomainValidationOptions: newCertValidateOptions,
    });

    AWS.mock("ACM", "requestCertificate", requestCertificateFake);
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason.startsWith(
            "DescribeCertificate did not contain DomainValidationOptions after"
          )
        );
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          IsAliasEnabled: "true",
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
          SubjectAlternativeNames: testSubjectAlternativeNames,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          requestCertificateFake,
          sinon.match({
            DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
            SubjectAlternativeNames: testSubjectAlternativeNames,
            ValidationMethod: "DNS",
          })
        );
        expect(request.isDone()).toBe(true);
        const totalSleep = spySleep
          .getCalls()
          .map((call) => call.args[0])
          .reduce((p, n) => p + n, 0);
        expect(totalSleep).toBeGreaterThan(60 * 1000);
      });
  });

  test("Create operation fails within 360s and 10 attempts if certificate has no DomainValidationOptions", () => {
    handler.withRandom(() => 1);
    const requestCertificateFake = sinon.fake.resolves({
      CertificateArn: testCertificateArn,
    });

    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
      },
    });

    AWS.mock("ACM", "requestCertificate", requestCertificateFake);
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason.startsWith(
            "DescribeCertificate did not contain DomainValidationOptions after"
          )
        );
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
          SubjectAlternativeNames: testSubjectAlternativeNames,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          requestCertificateFake,
          sinon.match({
            DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
            SubjectAlternativeNames: testSubjectAlternativeNames,
            ValidationMethod: "DNS",
          })
        );
        expect(request.isDone()).toBe(true);
        expect(spySleep.callCount).toBe(10);
        const totalSleep = spySleep
          .getCalls()
          .map((call) => call.args[0])
          .reduce((p, n) => p + n, 0);
        expect(totalSleep).toBeLessThan(360 * 1000);
      });
  });

  test("Create operation fails because no hosted zone found", () => {
    const requestCertificateFake = sinon.fake.resolves({
      CertificateArn: testCertificateArn,
    });

    const describeCertificateFake = sinon.stub();
    describeCertificateFake.onFirstCall().resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
      },
    });
    describeCertificateFake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        DomainValidationOptions: [
          {
            DomainName: `${testAppName}.${testDomainName}`,
            ValidationStatus: "SUCCESS",
            ResourceRecord: {
              Name: testRRName,
              Type: "CNAME",
              Value: testRRValue,
            },
          },
        ],
      },
    });

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [],
    });

    AWS.mock("ACM", "requestCertificate", requestCertificateFake);
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);
    AWS.mock(
      "Route53",
      "changeResourceRecordSets",
      changeResourceRecordSetsFake
    );
    AWS.mock("Route53", "listHostedZonesByName", listHostedZonesByNameFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason.startsWith("Couldn't find any Hosted Zone")
        );
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          IsAliasEnabled: "true",
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
          SubjectAlternativeNames: testSubjectAlternativeNames,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          requestCertificateFake,
          sinon.match({
            DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
            SubjectAlternativeNames: testSubjectAlternativeNames,
            ValidationMethod: "DNS",
          })
        );
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: `${testAppName}.${testDomainName}`,
            MaxItems: "1",
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation with a maximum of 1 attempts describes the certificate once", () => {
    handler.withMaxAttempts(1);

    const requestCertificateFake = sinon.fake.resolves({
      CertificateArn: testCertificateArn,
    });
    AWS.mock("ACM", "requestCertificate", requestCertificateFake);

    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        DomainValidationOptions: legacyCertValidatorOptions,
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });
    AWS.mock(
      "Route53",
      "changeResourceRecordSets",
      changeResourceRecordSetsFake
    );

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          IsAliasEnabled: "true",
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
          SubjectAlternativeNames: testSubjectAlternativeNames,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledOnce(describeCertificateFake);
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation deletes the last legacy cert", () => {
    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        DomainValidationOptions: legacyCertValidatorOptions,
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const deleteCertificateFake = sinon.fake.resolves({});
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const listCertificatesFake = sinon.stub();
    listCertificatesFake.onFirstCall().resolves({
      CertificateSummaryList: [
        {
          DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
        },
      ],
      NextToken: "some token",
    });
    listCertificatesFake.onSecondCall().resolves({
      CertificateSummaryList: [
        {
          DomainName: "otherdomain.com",
        },
      ],
    });
    AWS.mock("ACM", "listCertificates", listCertificatesFake);

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });
    AWS.mock(
      "Route53",
      "changeResourceRecordSets",
      changeResourceRecordSetsFake
    );

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.calledWith(listCertificatesFake, sinon.match({}));
        sinon.assert.calledWith(
          listCertificatesFake,
          sinon.match({
            NextToken: "some token",
          })
        );
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: testDeleteRecordChangebatch,
            HostedZoneId: testHostedZoneId,
          })
        );
        sinon.assert.calledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation deletes the legacy cert but not the last", () => {
    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        DomainValidationOptions: legacyCertValidatorOptions,
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const deleteCertificateFake = sinon.fake.resolves({});
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const listCertificatesFake = sinon.stub();
    listCertificatesFake.resolves({
      CertificateSummaryList: [
        {
          DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
        },
        {
          DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
        },
      ],
    });
    AWS.mock("ACM", "listCertificates", listCertificatesFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.calledWith(listCertificatesFake, sinon.match({}));
        sinon.assert.calledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation deletes the last new cert", () => {
    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        DomainValidationOptions: newCertValidateOptions,
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const deleteCertificateFake = sinon.fake.resolves({});
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const listCertificatesFake = sinon.stub();
    listCertificatesFake.resolves({
      CertificateSummaryList: [
        {
          DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
        },
      ],
    });
    AWS.mock("ACM", "listCertificates", listCertificatesFake);

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });
    AWS.mock(
      "Route53",
      "changeResourceRecordSets",
      changeResourceRecordSetsFake
    );

    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [
        {
          Id: `/hostedzone/${testAppHostedZoneId}`,
        },
      ],
    });
    AWS.mock("Route53", "listHostedZonesByName", listHostedZonesByNameFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.calledWith(listCertificatesFake, sinon.match({}));
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: testDeleteRecordChangebatch,
            HostedZoneId: testHostedZoneId,
          })
        );
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: testDeleteRecordChangebatch,
            HostedZoneId: testAppHostedZoneId,
          })
        );
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: `${testAppName}.${testDomainName}`,
            MaxItems: "1",
          })
        );
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: `${testDomainName}`,
            MaxItems: "1",
          })
        );
        sinon.assert.calledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation deletes the last new cert but not the last", () => {
    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        DomainValidationOptions: newCertValidateOptions,
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const deleteCertificateFake = sinon.fake.resolves({});
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const listCertificatesFake = sinon.stub();
    listCertificatesFake.resolves({
      CertificateSummaryList: [
        {
          DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
        },
        {
          DomainName: `${testEnvName}.${testAppName}.${testDomainName}`,
        },
      ],
    });
    AWS.mock("ACM", "listCertificates", listCertificatesFake);

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });
    AWS.mock(
      "Route53",
      "changeResourceRecordSets",
      changeResourceRecordSetsFake
    );

    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [
        {
          Id: `/hostedzone/${testAppHostedZoneId}`,
        },
      ],
    });
    AWS.mock("Route53", "listHostedZonesByName", listHostedZonesByNameFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.calledWith(listCertificatesFake, sinon.match({}));
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: testDeleteRecordChangebatch,
            HostedZoneId: testAppHostedZoneId,
          })
        );
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: `${testAppName}.${testDomainName}`,
            MaxItems: "1",
          })
        );
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: `${testDomainName}`,
            MaxItems: "1",
          })
        );
        sinon.assert.calledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation is idempotent", () => {
    const error = new Error();
    error.name = "ResourceNotFoundException";

    const describeCertificateFake = sinon.fake.rejects(error);
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const deleteCertificateFake = sinon.fake.rejects(error);
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.neverCalledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation succeeds if certificate becomes not-in-use", () => {
    const usedByArn =
      "arn:aws:cloudfront::123456789012:distribution/d111111abcdef8";

    const describeCertificateFake = sinon.stub();
    describeCertificateFake.onFirstCall().resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        InUseBy: [usedByArn],
      },
    });

    describeCertificateFake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        InUseBy: [],
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const deleteCertificateFake = sinon.fake.resolves({});
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const listCertificatesFake = sinon.stub();
    listCertificatesFake.resolves({});
    AWS.mock("ACM", "listCertificates", listCertificatesFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.calledWith(listCertificatesFake, sinon.match({}));
        sinon.assert.calledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation fails within 360s and 10 attempts if certificate is in-use", () => {
    const usedByArn =
      "arn:aws:cloudfront::123456789012:distribution/d111111abcdef8";

    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
        InUseBy: [usedByArn],
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const error = new Error();
    error.name = "ResourceInUseException";
    const deleteCertificateFake = sinon.fake.rejects(error);
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.neverCalledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        const totalSleep = spySleep
          .getCalls()
          .map((call) => call.args[0])
          .reduce((p, n) => p + n, 0);
        expect(totalSleep).toBeLessThan(360 * 1000);
        expect(spySleep.callCount).toBe(10);
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation fails if some other error is encountered during describe", () => {
    const error = new Error();
    error.name = "SomeOtherException";

    const describeCertificateFake = sinon.fake.rejects(error);
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const deleteCertificateFake = sinon.fake.resolves({});
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.neverCalledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation fails if some other error is encountered during delete", () => {
    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const error = new Error();
    error.name = "SomeOtherException";
    const deleteCertificateFake = sinon.fake.rejects(error);
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const listCertificatesFake = sinon.stub();
    listCertificatesFake.resolves({});
    AWS.mock("ACM", "listCertificates", listCertificatesFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.calledWith(listCertificatesFake, sinon.match({}));
        sinon.assert.calledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation with a maximum of 1 attempts describes the certificate once", () => {
    handler.withMaxAttempts(1);
    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const deleteCertificateFake = sinon.fake.resolves({});
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const listCertificatesFake = sinon.stub();
    listCertificatesFake.resolves({});
    AWS.mock("ACM", "listCertificates", listCertificatesFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateRequestHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          DomainName: testDomainName,
          EnvHostedZoneId: testHostedZoneId,
          Region: "us-east-1",
          RootDNSRole: testRootDNSRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        sinon.assert.calledWith(listCertificatesFake, sinon.match({}));
        sinon.assert.calledWith(
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });
});
