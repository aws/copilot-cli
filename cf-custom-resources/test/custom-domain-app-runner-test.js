// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

/* jshint node: true */
/*jshint esversion: 8 */
const AWS = require("aws-sdk-mock");
const LambdaTester = require("lambda-tester").noVersionCheck();
const handler = require("../lib/custom-domain-app-runner").handler;
const sinon = require("sinon");




describe("Custom Domain for App Runner Service", () => {
    const [mockServiceARN, mockCustomDomain, mockHostedZoneID] = ["mockService", "mockDomain", "mockHostedZoneID",];

    let mockTarget = "mockTarget";
    let mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
    let mockWaitFor = sinon.fake.resolves();
    let mockDescribeCustomDomains = sinon.fake.resolves({
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
            },
        ],
    });
    let mockChangeResourceRecordSets = sinon.stub();

    function setUpMocks() {
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
    }

    beforeEach(() => {
    });

    afterEach(() => {
        // Restore waiters and logger
        AWS.restore();
    });

    test("fail to associate custom domain", () => {
        const mockAssociateCustomDomain = sinon.fake.rejects(new Error("some error"));
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        return LambdaTester( handler )
            .event({
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectReject( err => {
                expect(err.message).toBe("add custom domain mockDomain: some error");
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

        return LambdaTester( handler )
            .event({
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectReject( err => {
                expect(err.message).toBe("add custom domain mockDomain: upsert record mockDomain: some error");
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

        return LambdaTester( handler )
            .event({
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectReject( err => {
                expect(err.message).toBe("add custom domain mockDomain: wait for record sets change for mockDomain: some error");
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

        return LambdaTester( handler )
            .event({
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectReject( err => {
                expect(err.message).toBe("add custom domain mockDomain: get custom domains for service mockService: some error");
                sinon.assert.calledWith(mockDescribeCustomDomains, sinon.match({ ServiceArn: mockServiceARN, }));
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

        return LambdaTester( handler )
            .event({
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectReject( err => {
                expect(err.message).toBe("add custom domain mockDomain: upsert record mock-record-name-2: some error");
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

    test("success", () => {
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
                },
            ],
        });
        const mockChangeResourceRecordSets = sinon.stub();
        mockChangeResourceRecordSets.resolves({ ChangeInfo: {Id: "mockID", }, });

        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        return LambdaTester( handler )
            .event({
                ResourceProperties: {
                    ServiceARN: mockServiceARN,
                    AppDNSRole: "",
                    CustomDomain: mockCustomDomain,
                    HostedZoneID: mockHostedZoneID,
                },
            })
            .expectResolve();
    });
});