// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const AWS = require("aws-sdk-mock");
const LambdaTester = require("lambda-tester").noVersionCheck();
const sinon = require("sinon");
const nock = require("nock");
let origLog = console.log;

const {handler, withSleep, withDeadlineExpired, reset, attemptsValidationOptionsReady} = require("../lib/nlb-cert-validator-updater");

describe("DNS Certificate Validation And Custom Domains for NLB", () => {
    // Mock requests.
    const mockServiceName = "web";
    const mockEnvName = "mockEnv";
    const mockAppName = "mockApp";
    const mockDomainName = "mockDomain.com";
    const mockEnvHostedZoneID = "mockEnvHostedZoneID";
    const mockLBDNS = "mockLBDNS";
    const mockLBHostedZoneID = "mockLBHostedZoneID"
    const mockResponseURL = "https://mock.com/";
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

    // Mock respond request.
    function mockFailedRequest(expectedErrMessageRegex) {
        return nock(mockResponseURL)
            .put("/", (body) => {
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1
                );
            })
            .reply(200);
    }

    // API call mocks.
    const mockListResourceRecordSets = sinon.stub();
    const mockRequestCertificate = sinon.stub();
    const mockDescribeCertificate = sinon.stub();
    const mockChangeResourceRecordSets = sinon.stub();
    beforeEach(() => {
        // Prevent logging.
        console.log = function () {};
        withSleep(_ => {
            return Promise.resolve();
        });
        withDeadlineExpired(_ => {
            return new Promise(function (resolve, reject) {});
        });

        // Mock API default behavior.
        mockListResourceRecordSets.resolves({
            "ResourceRecordSets": []
        });
        mockRequestCertificate.resolves({
            "CertificateArn": "mockCertArn",
        });
        mockDescribeCertificate.resolves({
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
        mockChangeResourceRecordSets.resolves({ ChangeInfo: {Id: "mockChangeID", }, })
    });

    afterEach(() => {
        // Restore logger
        console.log = origLog;
        AWS.restore();
        reset();

        // Reset mocks call count.
        mockListResourceRecordSets.reset();
        mockRequestCertificate.reset();
        mockDescribeCertificate.reset();
        mockChangeResourceRecordSets.reset();
    });

    describe("During CREATE with alias", () => {
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
                "ResourceRecordSets": [{
                    "AliasTarget": {
                        "DNSName": "other-lb-DNS",
                    }
                }]
            });
            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            let request = mockFailedRequest(/^Alias dash-test.mockDomain.com is in use by other-lb-DNS. This could be another load balancer of a different service. \(Log: .*\)$/);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.callCount(mockListResourceRecordSets, 3);
                });
        });

        test("fail to request a certificate", () => {
            const mockRequestCertificate =sinon.fake.rejects(new Error("some error"));
            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);

            let request = mockFailedRequest(/^some error \(Log: .*\)$/);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.callCount(mockListResourceRecordSets, 3);
                    sinon.assert.callCount(mockRequestCertificate, 1);
                });
        })

        test("timed out waiting for validation options to be ready", () => {
            const mockDescribeCertificate = sinon.fake.resolves({
                "Certificate": {
                    "DomainValidationOptions": [{
                        "ResourceRecord": {},
                        "DomainName": "not the domain we want",
                    }],
                },
            });

            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
            AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);

            let request = mockFailedRequest(/^resource validation records are not ready after 10 tries \(Log: .*\)$/);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.callCount(mockListResourceRecordSets, 3);
                    sinon.assert.callCount(mockRequestCertificate, 1);
                    sinon.assert.callCount(mockDescribeCertificate, attemptsValidationOptionsReady);
                });
        });

        test("error while waiting for validation options to be ready", () => {
            const mockDescribeCertificate = sinon.fake.rejects(new Error("some error"));

            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
            AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);

            let request = mockFailedRequest(/^some error \(Log: .*\)$/);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.callCount(mockListResourceRecordSets, 3);
                    sinon.assert.callCount(mockRequestCertificate, 1);
                    sinon.assert.callCount(mockDescribeCertificate, 1);
                });
        });

        test("fail to upsert validation record and alias A-record for an alias into hosted zone", () => {
            const mockChangeResourceRecordSets = sinon.stub();
            mockChangeResourceRecordSets.withArgs(sinon.match.hasNested("ChangeBatch.Changes[1].ResourceRecordSet.Name", "dash-test.mockDomain.com")).rejects(new Error("some error"));
            mockChangeResourceRecordSets.resolves({ChangeInfo: {Id: "mockID",},});

            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);
            AWS.mock("ACM", "requestCertificate", mockRequestCertificate);
            AWS.mock("ACM", "describeCertificate", mockDescribeCertificate);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);

            let request = mockFailedRequest(/^some error \(Log: .*\)$/);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.callCount(mockListResourceRecordSets, 3);
                    sinon.assert.callCount(mockRequestCertificate, 1);
                    sinon.assert.callCount(mockDescribeCertificate, 1);
                    sinon.assert.callCount(mockChangeResourceRecordSets, 4);
                });
        });

        test("fail to wait for resource record sets change to be finished", () => {
            const mockWaitFor = sinon.fake.rejects(new Error("some error"));

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
            mockWaitForCertificateValidation.withArgs('certificateValidated', sinon.match.has("CertificateArn", "mockCertArn")).rejects(new Error("some error"));

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
                    sinon.assert.callCount(mockListResourceRecordSets, 3);
                    sinon.assert.callCount(mockRequestCertificate, 1);
                    sinon.assert.callCount(mockDescribeCertificate, 1);
                    sinon.assert.callCount(mockChangeResourceRecordSets, 4);
                    sinon.assert.callCount(mockWaitForRecordsChange, 4);
                    sinon.assert.callCount(mockWaitForCertificateValidation, 1);
                });
        });
    })
});
