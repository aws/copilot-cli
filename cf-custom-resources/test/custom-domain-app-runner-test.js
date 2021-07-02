// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

/* jshint node: true */
/*jshint esversion: 8 */


const AWS = require("aws-sdk-mock");
const LambdaTester = require("lambda-tester").noVersionCheck();
const {handler, domainStatusPendingVerification, waitForDomainStatusChangeAttempts, withSleep, reset} = require("../lib/custom-domain-app-runner");
const sinon = require("sinon");
const nock = require("nock");

describe("Custom Domain for App Runner Service During Create", () => {
    const [mockServiceARN, mockCustomDomain, mockHostedZoneID] = ["mockService", "mockDomain", "mockHostedZoneID",];
    const mockResponseURL = "https://mock.com/";

    beforeEach(() => {
        withSleep(_ => {
            return new Promise((resolve) => setTimeout(resolve, 1));
        });
    });

    afterEach(() => {
        AWS.restore();
        reset();
    });

    test("fail to associate custom domain", () => {
        const mockAssociateCustomDomain = sinon.fake.rejects(new Error("some error"));
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {

                let expectedErrMessageRegex = /^some error \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1
                );
            })
            .reply(200);
        return LambdaTester( handler )
            .event({
                RequestType: "Create",
                ResponseURL: mockResponseURL,
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectResolve( () => {
                expect(expectedResponse.isDone()).toBe(true);
                sinon.assert.calledWith(mockAssociateCustomDomain, sinon.match({
                    DomainName: mockCustomDomain,
                    ServiceArn: mockServiceARN,
                }));
            });
    });

    test("fail to add the record for custom domain", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockChangeResourceRecordSets = sinon.fake.rejects(new Error("some error"));
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^upsert record mockDomain: some error \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1
                );
            })
            .reply(200);
        return LambdaTester( handler )
            .event({
                RequestType: "Create",
                ResponseURL: mockResponseURL,
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectResolve( () => {
                expect(expectedResponse.isDone()).toBe(true);
                sinon.assert.calledOnce(mockAssociateCustomDomain);
                sinon.assert.calledWith(mockChangeResourceRecordSets, sinon.match({
                    ChangeBatch: {
                        Changes: [
                            {
                                Action: "UPSERT",
                                ResourceRecordSet: {
                                    Name: mockCustomDomain,
                                    Type: "CNAME",
                                    TTL: 60,
                                    ResourceRecords: [
                                        {
                                            Value: mockTarget,
                                        },
                                    ],
                                },
                            },
                        ],
                    },
                    HostedZoneId: mockHostedZoneID,
                }));
            });
    });

    test("fail to wait for the custom domain record to change", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockChangeResourceRecordSets = sinon.fake.resolves({ ChangeInfo: {Id: "mockID", }, });
        const mockWaitFor = sinon.fake.rejects(new Error("some error"));
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^wait for record sets change for mockDomain: some error \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1
                );
            })
            .reply(200);
        return LambdaTester( handler )
            .event({
                RequestType: "Create",
                ResponseURL: mockResponseURL,
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectResolve( err => {
                expect(expectedResponse.isDone()).toBe(true);
                sinon.assert.calledOnce(mockAssociateCustomDomain);
                sinon.assert.calledWith(mockChangeResourceRecordSets, sinon.match({
                    ChangeBatch: {
                        Changes: [
                            {
                                Action: "UPSERT",
                                ResourceRecordSet: {
                                    Name: mockCustomDomain,
                                    Type: "CNAME",
                                    TTL: 60,
                                    ResourceRecords: [
                                        {
                                            Value: mockTarget,
                                        },
                                    ],
                                },
                            },
                        ],
                    },
                    HostedZoneId: mockHostedZoneID,
                }));
            });
    });

    test("fail to describe custom domain", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockChangeResourceRecordSets = sinon.fake.resolves({ ChangeInfo: {Id: "mockID", }, });
        const mockWaitFor = sinon.fake.resolves();
        const mockDescribeCustomDomains = sinon.fake.rejects(new Error("some error"));
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^get custom domains for service mockService: some error \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1
                );
            })
            .reply(200);
        return LambdaTester( handler )
            .event({
                RequestType: "Create",
                ResponseURL: mockResponseURL,
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectResolve( err => {
                expect(expectedResponse.isDone()).toBe(true);
                sinon.assert.calledWith(mockDescribeCustomDomains, sinon.match({ ServiceArn: mockServiceARN, }));
            });
    });

    test("fail to wait for app runner to provide validation records", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockWaitFor = sinon.fake.resolves();
        const mockChangeResourceRecordSets = sinon.fake.resolves({ ChangeInfo: {Id: "mockID", }, });
        const mockDescribeCustomDomains = sinon.stub();
        for (let i = 0; i < waitForDomainStatusChangeAttempts; i++) {
            mockDescribeCustomDomains.onCall(i).resolves({
                CustomDomains: [
                    {
                        DomainName: mockCustomDomain,
                        Status: "not-pending",
                    },
                ],
            });
        }

        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets",mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^failed waiting for custom domain mockDomain to change to state pending_certificate_dns_validation \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1
                );
            })
            .reply(200);
        return LambdaTester( handler )
            .event({
                RequestType: "Create",
                ResponseURL: mockResponseURL,
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectResolve( () => {
                expect(expectedResponse.isDone()).toBe(true);
            });
    });

    test("fail to add cert validation record", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockWaitFor = sinon.fake.resolves();
        const mockDescribeCustomDomains = sinon.fake.resolves({
            CustomDomains: [
                {
                    DomainName: "other-domain",
                    CertificateValidationRecords: [
                        {
                            Name: "this shouldn't appear",
                            Value: "this shouldn't appear",
                        },
                    ],
                },
                {
                    DomainName: mockCustomDomain,
                    CertificateValidationRecords: [
                        {
                            Name: "mock-record-name-1",
                            Value: "mock-record-value-1",
                        },
                        {
                            Name: "mock-record-name-2",
                            Value: "mock-record-value-2",
                        },
                    ],
                    Status: "pending_certificate_dns_validation",
                },
            ],
        });
        const mockChangeResourceRecordSets = sinon.stub();
        mockChangeResourceRecordSets.onCall(0).resolves({ ChangeInfo: {Id: "mockID", }, });
        mockChangeResourceRecordSets.onCall(1).resolves({ ChangeInfo: {Id: "mockID", }, });
        mockChangeResourceRecordSets.onCall(2).rejects(new Error("some error"));

        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^upsert certificate validation record: upsert record mock-record-name-2: some error \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1
                );
            })
            .reply(200);
        return LambdaTester( handler )
            .event({
                RequestType: "Create",
                ResponseURL: mockResponseURL,
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectResolve( () => {
                expect(expectedResponse.isDone()).toBe(true);
                sinon.assert.calledWith(mockChangeResourceRecordSets, sinon.match({
                    ChangeBatch: {
                        Changes: [
                            {
                                Action: "UPSERT",
                                ResourceRecordSet: {
                                    Name: "mock-record-name-1",
                                    Type: "CNAME",
                                    TTL: 60,
                                    ResourceRecords: [
                                        {
                                            Value: "mock-record-value-1",
                                        },
                                    ],
                                },
                            },
                        ],
                    },
                    HostedZoneId: mockHostedZoneID,
                }));
                sinon.assert.calledWith(mockChangeResourceRecordSets, sinon.match({
                    ChangeBatch: {
                        Changes: [
                            {
                                Action: "UPSERT",
                                ResourceRecordSet: {
                                    Name: "mock-record-name-2",
                                    Type: "CNAME",
                                    TTL: 60,
                                    ResourceRecords: [
                                        {
                                            Value: "mock-record-value-2",
                                        },
                                    ],
                                },
                            },
                        ],
                    },
                    HostedZoneId: mockHostedZoneID,
                }));
            });
    });

    test("fail to send failure response", () => {
        const mockAssociateCustomDomain = sinon.fake.rejects(new Error("some error"));
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        return LambdaTester( handler )
            .event({
                ResponseURL: "super weird URL",
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectReject(() => {});
    });

    test("success", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockWaitFor = sinon.fake.resolves();
        const mockDescribeCustomDomains = sinon.stub();

        // Successfully wait for custom domain's status to be "pending" after several waits.
        for (let i = 0; i < waitForDomainStatusChangeAttempts - 1; i++) {
            mockDescribeCustomDomains.onCall(i).resolves({
                CustomDomains: [
                    {
                        DomainName: mockCustomDomain,
                        Status: "not-pending",
                    },
                ],
            });
        }
        mockDescribeCustomDomains.onCall(waitForDomainStatusChangeAttempts - 1).resolves({
            CustomDomains: [
                {
                    DomainName: mockCustomDomain,
                    CertificateValidationRecords: [
                        {
                            Name: "mock-record-name-1",
                            Value: "mock-record-value-1",
                        },
                        {
                            Name: "mock-record-name-2",
                            Value: "mock-record-value-2",
                        },
                    ],
                    Status: domainStatusPendingVerification,
                },
            ],
        });

        const mockChangeResourceRecordSets = sinon.stub();
        mockChangeResourceRecordSets.resolves({ ChangeInfo: {Id: "mockID", }, });

        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                return body.Status === "SUCCESS";
            })
            .reply(200);
        return LambdaTester( handler )
            .event({
                RequestType: "Create",
                ResponseURL: mockResponseURL,
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectResolve(() => {
                expect(expectedResponse.isDone()).toBe(true);
            });
    });
});