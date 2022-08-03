// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

describe("Certificate Replicator Handler", () => {
  const AWS = require("aws-sdk-mock");
  const LambdaTester = require("lambda-tester").noVersionCheck();
  const sinon = require("sinon");
  const handler = require("../lib/cert-replicator");
  const nock = require("nock");
  const ResponseURL = "https://cloudwatch-response-mock.example.com/";
  const LogGroup = "/aws/lambda/testLambda";
  const LogStream = "2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd";

  let origLog = console.log;
  const testRequestId = "f4ef1b10-c39a-44e3-99c0-fbf7e53c3943";
  const testAppName = "myapp";
  const testEnvName = "test";
  const testDomainName = "example.com";
  const testSANs = [
    "test.myapp.example.com",
    "*.test.myapp.example.com",
    "v1.myapp.example.com",
    "v2.example.com",
  ];
  const testCopilotTags = [
    { Key: "copilot-application", Value: testAppName },
    { Key: "copilot-environment", Value: testEnvName },
  ];
  const testCertificateArn =
    "arn:aws:acm:region:123456789012:certificate/12345678-1234-1234-1234-123456789012";
  const otherCertificateArn =
    "arn:aws:acm:region:210987654322:certificate/87654421-1234-1234-1234-210987654321";
  const spySleep = sinon.spy(function (ms) {
    return Promise.resolve();
  });

  beforeEach(() => {
    handler.withDefaultResponseURL(ResponseURL);
    handler.withDefaultLogGroup(LogGroup);
    handler.withDefaultLogStream(LogStream);
    handler.withSleep(spySleep);
    handler.withDeadlineExpired((_) => {
      return new Promise(function (resolve, reject) {});
    });
    console.log = function () {};
  });
  afterEach(() => {
    handler.reset();
    AWS.restore();
    console.log = origLog;
    spySleep.resetHistory();
  });

  test("Empty event payload type fails", () => {
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason ===
            "Unsupported request type undefined (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd)" &&
          body.PhysicalResourceId === LogStream
        );
      })
      .reply(200);
    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
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
          body.Reason ===
            "Unsupported request type " +
              bogusType +
              " (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd)"
        );
      })
      .reply(200);
    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestType: bogusType,
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
        },
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation fails if cannot describe old certificate", () => {
    const describeCertificateFake = sinon.fake.rejects(new Error("some error"));

    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
        },
        OldResourceProperties: {},
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          describeCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation fails if cannot request new certificate", () => {
    const requestCertificateFake = sinon.fake.rejects(new Error("some error"));

    const describeCertificateFake = sinon.stub();
    describeCertificateFake.resolves({
      Certificate: {
        CertificateArn: otherCertificateArn,
        DomainName: testDomainName,
        SubjectAlternativeNames: testSANs,
      },
    });

    AWS.mock("ACM", "requestCertificate", requestCertificateFake);
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
        },
        OldResourceProperties: {},
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
            DomainName: testDomainName,
            SubjectAlternativeNames: testSANs,
            ValidationMethod: "DNS",
            Tags: testCopilotTags,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Update operation requests a new certificate", () => {
    const requestCertificateFake = sinon.fake.resolves({
      CertificateArn: otherCertificateArn,
    });

    const describeCertificateFake = sinon.stub();
    describeCertificateFake.resolves({
      Certificate: {
        CertificateArn: otherCertificateArn,
        DomainName: testDomainName,
        SubjectAlternativeNames: testSANs,
      },
    });

    const waitForCertificateValidationFake = sinon.stub();
    waitForCertificateValidationFake.resolves();

    AWS.mock("ACM", "requestCertificate", requestCertificateFake);
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);
    AWS.mock("ACM", "waitFor", waitForCertificateValidationFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "SUCCESS" &&
          body.PhysicalResourceId === otherCertificateArn
        );
      })
      .reply(200);

    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestType: "Update",
        RequestId: testRequestId,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
        },
        OldResourceProperties: {},
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
            DomainName: testDomainName,
            SubjectAlternativeNames: testSANs,
            ValidationMethod: "DNS",
            Tags: testCopilotTags,
          })
        );
        sinon.assert.calledWith(
          waitForCertificateValidationFake,
          "certificateValidated",
          sinon.match({
            $waiter: { delay: 30, maxAttempts: 19 },
            CertificateArn: otherCertificateArn,
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

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
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

    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
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

    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
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

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
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
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation with a maximum of 1 attempts and proceed if ResourceNotFoundException", () => {
    handler.withMaxAttempts(1);
    const describeCertificateFake = sinon.fake.resolves({
      Certificate: {
        CertificateArn: testCertificateArn,
      },
    });
    AWS.mock("ACM", "describeCertificate", describeCertificateFake);

    let resourceNotFoundErr = new Error("some error");
    resourceNotFoundErr.name = "ResourceNotFoundException";
    const deleteCertificateFake = sinon.fake.rejects(resourceNotFoundErr);
    AWS.mock("ACM", "deleteCertificate", deleteCertificateFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(handler.certificateReplicateHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        PhysicalResourceId: testCertificateArn,
        ResourceProperties: {
          AppName: testAppName,
          EnvName: testEnvName,
          TargetRegion: "us-east-1",
          EnvRegion: "us-west-2",
          CertificateArn: testCertificateArn,
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
          deleteCertificateFake,
          sinon.match({
            CertificateArn: testCertificateArn,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });
});
