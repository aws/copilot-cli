// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const AWS = require("aws-sdk-mock");
const LambdaTester = require("lambda-tester").noVersionCheck();
const sinon = require("sinon");
const nock = require("nock");
let origLog = console.log;

const {handler, withSleep, withDeadlineExpired, reset} = require("../lib/nlb-cert-validator-updater");


describe("DNS Certificate Validation And Custom Domains for NLB", () => {
    const mockResponseURL = "https://mock.com/";
    const mockServiceName = "web";
    const mockEnvName = "mockEnv";
    const mockAppName = "mockApp";
    const mockDomainName = "mockDomain.com";
    const mockEnvHostedZoneID = "mockEnvHostedZoneID";
    const mockLBDNS = "mockLBDNS";
    const mockLBHostedZoneID = "mockLBHostedZoneID"
    const mockRequest = {
        ResponseURL: mockResponseURL,
        ResourceProperties: {
            ServiceName: mockServiceName,
            Aliases: ["dash-test.mockDomain.com", "frontend.mockDomain.com", "frontend.v2.mockDomain.com"],
            EnvName: mockEnvName,
            AppName: mockAppName,
            DomainName: mockDomainName,
            LoadBalancerDNS: mockLBDNS,
            LoadBalancerHostedZoneID: mockLBHostedZoneID,
            EnvHostedZoneId: mockEnvHostedZoneID,
        },
        RequestType: "Create",
        LogicalResourceId: "mockID",
    };

    beforeEach(() => {
        // Prevent logging.
        console.log = function () {};
        withSleep(_ => {
            return Promise.resolve();
        });
        withDeadlineExpired(_ => {
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
        test("unsupported action fails", () => {
            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^Unsupported request type UNKNWON \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                }).reply(200);
            return LambdaTester(handler)
                .event({
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceName: "web",
                        Aliases: ["dash-test.mockDomain.com", "frontend.mockDomain.com", "frontend.v2.mockDomain.com"],
                        EnvName: "mockEnv",
                        AppName: "mockApp",
                        DomainName: "mockDomain.com",
                        LoadBalancerDNS: "mockLBDNS",
                        LoadBalancerHostedZoneID: "mockHostedZoneID",
                    },
                    RequestType: "UNKNWON",
                    LogicalResourceId: "mockID",
                })
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                });
        });

        test("correctly set physical resource ID", () => {
            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    return (
                        body.PhysicalResourceId === "/web/dash-test.mockDomain.com,frontend.mockDomain.com,frontend.v2.mockDomain.com"
                    );
                }).reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                });
        });

        test("error validating aliases", () => {
            const mockListResourceRecordSets = sinon.fake.rejects(new Error("some error"));
            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);

            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^some error \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.calledWith(mockListResourceRecordSets, sinon.match({
                        HostedZoneId: "mockEnvHostedZoneID",
                        MaxItems: "1",
                        StartRecordName: 'dash-test.mockDomain.com' // NOTE: JS set has the same iteration order: insertion order.
                    }));
                });
        });

        test("some aliases are in use by other service", () => {
            const mockListResourceRecordSets = sinon.fake.resolves({
                "ResourceRecordSets": [{
                    "AliasTarget": {
                        "DNSName": "other-lb-DNS",
                    }
                }]
            });
            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^alias dash-test.mockDomain.com is in use \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.calledWith(mockListResourceRecordSets, sinon.match({
                        HostedZoneId: "mockEnvHostedZoneID",
                        MaxItems: "1",
                        StartRecordName: 'dash-test.mockDomain.com' // NOTE: JS set has the same iteration order: insertion order.
                    }));
                });
        });

        test("fail to request a certificate", () => {
            const mockListResourceRecordSets = sinon.fake.resolves({
                "ResourceRecordSets": []
            });
            const mockRequestCertificate =sinon.fake.rejects(new Error("some error"));
            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);

            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^some error \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.calledWith(mockListResourceRecordSets, sinon.match({
                        HostedZoneId: "mockEnvHostedZoneID",
                        MaxItems: "1",
                        StartRecordName: 'dash-test.mockDomain.com' // NOTE: JS set has the same iteration order: insertion order.
                    }));
                    sinon.assert.calledWith(mockRequestCertificate, sinon.match({
                        DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
                        IdempotencyToken: "/web/dash-test.mockDomain.com,frontend.mockDomain.com,frontend.v2.mockDomain.com",
                        SubjectAlternativeNames: ["dash-test.mockDomain.com","frontend.mockDomain.com","frontend.v2.mockDomain.com"],
                        Tags: [
                            {
                                Key: "copilot-application",
                                Value: mockAppName,
                            },
                            {
                                Key: "copilot-environment",
                                Value: mockEnvName,
                            },
                        ],
                        ValidationMethod: "DNS",
                    }));
                });
        })

        test("timed out waiting for validation options to be ready", () => {
            const mockListResourceRecordSets = sinon.fake.resolves({
                "ResourceRecordSets": []
            });
            const mockRequestCertificate =sinon.fake.resolves({
                "CertificateArn": "mockCertArn",
            });
            const mockDescribeCertificate = sinon.fake.resolves({
                "Certificate": {
                    "DomainValidationOptions": [{
                        "ResourceRecord": {},
                        "DomainName": "not the domain want",
                    }],
                },
            });

            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
            AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);

            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^resource validation records are not ready after 10 tries \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.calledWith(mockListResourceRecordSets, sinon.match({
                        HostedZoneId: "mockEnvHostedZoneID",
                        MaxItems: "1",
                        StartRecordName: 'dash-test.mockDomain.com' // NOTE: JS set has the same iteration order: insertion order.
                    }));
                    sinon.assert.calledWith(mockRequestCertificate, sinon.match({
                        DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
                        IdempotencyToken: "/web/dash-test.mockDomain.com,frontend.mockDomain.com,frontend.v2.mockDomain.com",
                        SubjectAlternativeNames: ["dash-test.mockDomain.com","frontend.mockDomain.com","frontend.v2.mockDomain.com"],
                        Tags: [
                            {
                                Key: "copilot-application",
                                Value: mockAppName,
                            },
                            {
                                Key: "copilot-environment",
                                Value: mockEnvName,
                            },
                        ],
                        ValidationMethod: "DNS",
                    }));
                    sinon.assert.calledWith(mockDescribeCertificate, sinon.match({
                        "CertificateArn": "mockCertArn",
                    }));
                });
        });

        test("error while waiting for validation options to be ready", () => {
            const mockListResourceRecordSets = sinon.fake.resolves({
                "ResourceRecordSets": []
            });
            const mockRequestCertificate =sinon.fake.resolves({
                "CertificateArn": "mockCertArn",
            });
            const mockDescribeCertificate = sinon.fake.rejects(new Error("some error"));

            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
            AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);

            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^some error \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.calledWith(mockListResourceRecordSets, sinon.match({
                        HostedZoneId: "mockEnvHostedZoneID",
                        MaxItems: "1",
                        StartRecordName: 'dash-test.mockDomain.com' // NOTE: JS set has the same iteration order: insertion order.
                    }));
                    sinon.assert.calledWith(mockRequestCertificate, sinon.match({
                        DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
                        IdempotencyToken: "/web/dash-test.mockDomain.com,frontend.mockDomain.com,frontend.v2.mockDomain.com",
                        SubjectAlternativeNames: ["dash-test.mockDomain.com","frontend.mockDomain.com","frontend.v2.mockDomain.com"],
                        Tags: [
                            {
                                Key: "copilot-application",
                                Value: mockAppName,
                            },
                            {
                                Key: "copilot-environment",
                                Value: mockEnvName,
                            },
                        ],
                        ValidationMethod: "DNS",
                    }));
                    sinon.assert.calledWith(mockDescribeCertificate, sinon.match({
                        "CertificateArn": "mockCertArn",
                    }));
                });
        });

        test("fail to upsert validation record and alias A-record for an alias into environment hosted zone", () => {
            const mockListResourceRecordSets = sinon.fake.resolves({
                "ResourceRecordSets": []
            });
            const mockRequestCertificate =sinon.fake.resolves({
                "CertificateArn": "mockCertArn",
            });
            const mockDescribeCertificate = sinon.fake.resolves({
                "Certificate": {
                    "DomainValidationOptions": [{
                        "ResourceRecord": {
                            Name: "mock-validate-default-cert",
                            Value: "mock-validate-default-cert-value",
                            Type: "mock-validate-default-cert-type"
                        },
                        "DomainName": `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
                    },{
                        "ResourceRecord": {
                            Name: "mock-validate-alias-1",
                            Value: "mock-validate-alias-1-value",
                            Type: "mock-validate-alias-1-type"
                        },
                        "DomainName": "dash-test.mockDomain.com",
                    },{
                        "ResourceRecord": {
                            Name: "mock-validate-alias-2",
                            Value: "mock-validate-alias-2-value",
                            Type: "mock-validate-alias-2-type"
                        },
                        "DomainName": "frontend.mockDomain.com",
                    },{
                        "ResourceRecord": {
                            Name: "mock-validate-alias-3",
                            Value: "mock-validate-alias-3-value",
                            Type: "mock-validate-alias-3-type"
                        },
                        "DomainName": "frontend.v2.mockDomain.com",
                    }],
                },
            });
            const mockChangeResourceRecordSets = sinon.stub();
            mockChangeResourceRecordSets.onCall(0).resolves({ChangeInfo: {Id: "mockID",},});
            mockChangeResourceRecordSets.onCall(1).rejects(new Error("some error"));

            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
            AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);

            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^some error \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.calledWith(mockListResourceRecordSets, sinon.match({
                        HostedZoneId: "mockEnvHostedZoneID",
                        MaxItems: "1",
                        StartRecordName: 'dash-test.mockDomain.com' // NOTE: JS set has the same iteration order: insertion order.
                    }));
                    sinon.assert.calledWith(mockRequestCertificate, sinon.match({
                        DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
                        IdempotencyToken: "/web/dash-test.mockDomain.com,frontend.mockDomain.com,frontend.v2.mockDomain.com",
                        SubjectAlternativeNames: ["dash-test.mockDomain.com","frontend.mockDomain.com","frontend.v2.mockDomain.com"],
                        Tags: [
                            {
                                Key: "copilot-application",
                                Value: mockAppName,
                            },
                            {
                                Key: "copilot-environment",
                                Value: mockEnvName,
                            },
                        ],
                        ValidationMethod: "DNS",
                    }));
                    sinon.assert.calledWith(mockDescribeCertificate, sinon.match({
                        "CertificateArn": "mockCertArn",
                    }));
                    sinon.assert.calledWith(mockChangeResourceRecordSets, sinon.match({
                        ChangeBatch: {
                            Changes: [
                                {
                                    Action: "UPSERT",
                                    ResourceRecordSet: {
                                        Name: "mock-validate-default-cert",
                                        Type: "mock-validate-default-cert-type",
                                        TTL: 60,
                                        ResourceRecords: [
                                            {
                                                Value: "mock-validate-default-cert-value",
                                            },
                                        ],
                                    },
                                },
                            ],
                        },
                        HostedZoneId: mockEnvHostedZoneID,
                    }));
                    sinon.assert.calledWith(mockChangeResourceRecordSets, sinon.match({
                        ChangeBatch: {
                            Changes: [
                                {
                                    Action: "UPSERT",
                                    ResourceRecordSet: {
                                        Name: "mock-validate-alias-1",
                                        Type: "mock-validate-alias-1-type",
                                        TTL: 60,
                                        ResourceRecords: [
                                            {
                                                Value: "mock-validate-alias-1-value",
                                            },
                                        ],
                                    },
                                },
                                {
                                    Action: "UPSERT",
                                    ResourceRecordSet: {
                                        Name: "dash-test.mockDomain.com",
                                        Type: "A",
                                        AliasTarget:  {
                                            DNSName: mockLBDNS,
                                            EvaluateTargetHealth: true,
                                            HostedZoneId: mockLBHostedZoneID,
                                        }
                                    },
                                },
                            ],
                        },
                        HostedZoneId: mockEnvHostedZoneID,
                    }));
                });
        });

        test("fail to wait for resource record sets change to be finished", () => {
            const mockListResourceRecordSets = sinon.fake.resolves({
                "ResourceRecordSets": []
            });
            const mockRequestCertificate =sinon.fake.resolves({
                "CertificateArn": "mockCertArn",
            });
            const mockDescribeCertificate = sinon.fake.resolves({
                "Certificate": {
                    "DomainValidationOptions": [{
                        "ResourceRecord": {
                            Name: "mock-validate-default-cert",
                            Value: "mock-validate-default-cert-value",
                            Type: "mock-validate-default-cert-type"
                        },
                        "DomainName": `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
                    },{
                        "ResourceRecord": {
                            Name: "mock-validate-alias-1",
                            Value: "mock-validate-alias-1-value",
                            Type: "mock-validate-alias-1-type"
                        },
                        "DomainName": "dash-test.mockDomain.com",
                    },{
                        "ResourceRecord": {
                            Name: "mock-validate-alias-2",
                            Value: "mock-validate-alias-2-value",
                            Type: "mock-validate-alias-2-type"
                        },
                        "DomainName": "frontend.mockDomain.com",
                    },{
                        "ResourceRecord": {
                            Name: "mock-validate-alias-3",
                            Value: "mock-validate-alias-3-value",
                            Type: "mock-validate-alias-3-type"
                        },
                        "DomainName": "frontend.v2.mockDomain.com",
                    }],
                },
            });
            const mockChangeResourceRecordSets = sinon.fake.resolves({ ChangeInfo: {Id: "mockChangeID", }, }); // Resolves "mockValidationRecordID" for other calls.
            const mockWaitFor = sinon.stub();
            mockWaitFor.withArgs("resourceRecordSetsChanged", sinon.match.has("Id", "mockChangeID")).rejects(new Error("some error"));

            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
            AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);

            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^some error \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.calledWith(mockListResourceRecordSets, sinon.match({
                        HostedZoneId: "mockEnvHostedZoneID",
                        MaxItems: "1",
                        StartRecordName: 'dash-test.mockDomain.com' // NOTE: JS set has the same iteration order: insertion order.
                    }));
                    sinon.assert.calledWith(mockRequestCertificate, sinon.match({
                        DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
                        IdempotencyToken: "/web/dash-test.mockDomain.com,frontend.mockDomain.com,frontend.v2.mockDomain.com",
                        SubjectAlternativeNames: ["dash-test.mockDomain.com","frontend.mockDomain.com","frontend.v2.mockDomain.com"],
                        Tags: [
                            {
                                Key: "copilot-application",
                                Value: mockAppName,
                            },
                            {
                                Key: "copilot-environment",
                                Value: mockEnvName,
                            },
                        ],
                        ValidationMethod: "DNS",
                    }));
                    sinon.assert.calledWith(mockDescribeCertificate, sinon.match({
                        "CertificateArn": "mockCertArn",
                    }));
                    sinon.assert.callCount(mockChangeResourceRecordSets, 4);
                    sinon.assert.callCount(mockWaitFor, 4);
                });
        });

        test("fail to wait for certificate to be validated", () => {
            const mockListResourceRecordSets = sinon.fake.resolves({
                "ResourceRecordSets": []
            });
            const mockRequestCertificate =sinon.fake.resolves({
                "CertificateArn": "mockCertArn",
            });
            const mockDescribeCertificate = sinon.fake.resolves({
                "Certificate": {
                    "DomainValidationOptions": [{
                        "ResourceRecord": {
                            Name: "mock-validate-default-cert",
                            Value: "mock-validate-default-cert-value",
                            Type: "mock-validate-default-cert-type"
                        },
                        "DomainName": `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
                    },{
                        "ResourceRecord": {
                            Name: "mock-validate-alias-1",
                            Value: "mock-validate-alias-1-value",
                            Type: "mock-validate-alias-1-type"
                        },
                        "DomainName": "dash-test.mockDomain.com",
                    },{
                        "ResourceRecord": {
                            Name: "mock-validate-alias-2",
                            Value: "mock-validate-alias-2-value",
                            Type: "mock-validate-alias-2-type"
                        },
                        "DomainName": "frontend.mockDomain.com",
                    },{
                        "ResourceRecord": {
                            Name: "mock-validate-alias-3",
                            Value: "mock-validate-alias-3-value",
                            Type: "mock-validate-alias-3-type"
                        },
                        "DomainName": "frontend.v2.mockDomain.com",
                    }],
                },
            });
            const mockChangeResourceRecordSets = sinon.fake.resolves({ ChangeInfo: {Id: "mockChangeID", }, }); // Resolves "mockValidationRecordID" for other calls.
            const mockWaitForRecordsChange = sinon.stub();
            mockWaitForRecordsChange.withArgs("resourceRecordSetsChanged", sinon.match.has("Id", "mockChangeID")).resolves();
            const mockWaitForCertificateValidation = sinon.stub();
            mockWaitForCertificateValidation.withArgs('certificateValidated', sinon.match.has("CertificateArn", "mockCertArn")).rejects(new Error("some error"));

            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
            AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitForRecordsChange);
            AWS.mock("ACM", "waitFor", mockWaitForCertificateValidation);

            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^some error \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.calledWith(mockListResourceRecordSets, sinon.match({
                        HostedZoneId: "mockEnvHostedZoneID",
                        MaxItems: "1",
                        StartRecordName: 'dash-test.mockDomain.com' // NOTE: JS set has the same iteration order: insertion order.
                    }));
                    sinon.assert.calledWith(mockRequestCertificate, sinon.match({
                        DomainName: `${mockServiceName}-nlb.${mockEnvName}.${mockAppName}.${mockDomainName}`,
                        IdempotencyToken: "/web/dash-test.mockDomain.com,frontend.mockDomain.com,frontend.v2.mockDomain.com",
                        SubjectAlternativeNames: ["dash-test.mockDomain.com","frontend.mockDomain.com","frontend.v2.mockDomain.com"],
                        Tags: [
                            {
                                Key: "copilot-application",
                                Value: mockAppName,
                            },
                            {
                                Key: "copilot-environment",
                                Value: mockEnvName,
                            },
                        ],
                        ValidationMethod: "DNS",
                    }));
                    sinon.assert.calledWith(mockDescribeCertificate, sinon.match({
                        "CertificateArn": "mockCertArn",
                    }));
                    sinon.assert.callCount(mockChangeResourceRecordSets, 4);
                    sinon.assert.callCount(mockWaitForRecordsChange, 4);
                    sinon.assert.callCount(mockWaitForCertificateValidation, 1);
                });
        });
    })
});