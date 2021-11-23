// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const AWS = require("aws-sdk-mock");
const LambdaTester = require("lambda-tester").noVersionCheck();
const sinon = require("sinon");
const nock = require("nock");
let origLog = console.log;

const { attemptsValidationOptionsReady } = require("../lib/nlb-cert-validator-updater");

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
    const mockRootDNSRole = "mockRootDNSRole"
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
    const mockListHostedZonesByName = sinon.stub();
    const mockListResourceRecordSets = sinon.stub();
    const mockRequestCertificate = sinon.stub();
    const mockDescribeCertificate = sinon.stub();
    const mockChangeResourceRecordSets = sinon.stub();
    const mockAppHostedZoneID = "mockAppHostedZoneID";
    const mockRootHostedZoneID = "mockRootHostedZoneID";

    let handler, reset, withDeadlineExpired ;
    beforeEach(() => {
        // Prevent logging.
        console.log = function () {};

        // Reimport handlers so that the lazy loading does not fail the mocks.
        // A description of the issue can be found here: https://github.com/dwyl/aws-sdk-mock/issues/206.
        // This workaround follows the comment here: https://github.com/dwyl/aws-sdk-mock/issues/206#issuecomment-640418772.
        jest.resetModules();
        AWS.setSDKInstance(require('aws-sdk'));
        const imported = require("../lib/nlb-cert-validator-updater");
        handler = imported.handler;
        reset = imported.reset;
        withDeadlineExpired = imported.withDeadlineExpired;

        // Mocks wait functions.
        imported.withSleep(_ => {
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
                    "DomainName": "a.mockApp.mockDomain.com",
                },{
                    "ResourceRecord": {
                        Name: "mock-validate-alias-3",
                        Value: "mock-validate-alias-3-value",
                        Type: "mock-validate-alias-3-type"
                    },
                    "DomainName": "b.mockEnv.mockApp.mockDomain.com",
                }],
            },
        });
        mockChangeResourceRecordSets.resolves({ ChangeInfo: {Id: "mockChangeID", }, })
        mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockApp.mockDomain.com")).resolves({
            HostedZones: [{
                Id: mockAppHostedZoneID
            }]
        });
        mockListHostedZonesByName.withArgs(sinon.match.has("DNSName", "mockDomain.com")).resolves({
            HostedZones: [{
                Id: mockRootHostedZoneID,
            }]
        });
    });

    afterEach(() => {
        // Restore logger
        console.log = origLog;
        AWS.restore();
        reset();

        // Reset mocks call count.
        mockListHostedZonesByName.reset();
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
                        body.PhysicalResourceId === "/web/a.mockApp.mockDomain.com,b.mockEnv.mockApp.mockDomain.com,dash-test.mockDomain.com"
                    );
                }).reply(200);
            return LambdaTester(handler)
                .event(mockRequest)
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
                HostedZones: [{
                    Id: mockRootHostedZoneID,
                }]
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
                HostedZones: [{
                    Id: mockAppHostedZoneID,
                }]
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
                "ResourceRecordSets": [{
                    "AliasTarget": {
                        "DNSName": "other-lb-DNS",
                    }
                }]
            });
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
            AWS.mock("Route53", "listResourceRecordSets", mockListResourceRecordSets);

            let request = mockFailedRequest(/^Alias dash-test.mockDomain.com is already in use by other-lb-DNS. This could be another load balancer of a different service. \(Log: .*\)$/);
            return LambdaTester(handler)
                .event(mockRequest)
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.callCount(mockListHostedZonesByName, 2);
                    sinon.assert.callCount(mockListResourceRecordSets, 3);
                });
        });

        test("fail to request a certificate", () => {
            const mockRequestCertificate =sinon.fake.rejects(new Error("some error"));
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

        test("fail to upsert validation record and alias A-record for an alias into hosted zone", () => {
            const mockChangeResourceRecordSets = sinon.stub();
            mockChangeResourceRecordSets.withArgs(sinon.match.hasNested("ChangeBatch.Changes[1].ResourceRecordSet.Name", "dash-test.mockDomain.com")).rejects(new Error("some error"));
            mockChangeResourceRecordSets.resolves({ChangeInfo: {Id: "mockID",},});

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
            mockWaitForCertificateValidation.withArgs('certificateValidated', sinon.match.has("CertificateArn", "mockCertArn")).rejects(new Error("some error"));

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
            withDeadlineExpired(_ => {
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

    })
});
