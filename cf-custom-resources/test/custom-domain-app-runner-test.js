// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

/* jshint node: true */
/*jshint esversion: 8 */

const AWS = require("aws-sdk-mock");
const LambdaTester = require("lambda-tester").noVersionCheck();
const {handler, domainStatusPendingVerification, waitForDomainStatusPendingAttempts, waitForDomainStatusActiveAttempts, withSleep, reset, withDeadlineExpired} = require("../lib/custom-domain-app-runner");
const sinon = require("sinon");
const nock = require("nock");
let origLog = console.log;

describe("Custom Domain for App Runner Service During Create", () => {
    const [mockServiceARN, mockCustomDomain, mockHostedZoneID, mockResponseURL, mockPhysicalResourceID, mockLogicalResourceID] =
        ["mockService", "mockDomain", "mockHostedZoneID", "https://mock.com/", "mockPhysicalResourceID", "mockLogicalResourceID", ];

    beforeEach(() => {
        // Prevent logging.
        console.log = function () {};
        withSleep(_ => {
            return Promise.resolve();
        });
        withDeadlineExpired(_ => {
            return new Promise((resolve) => {
                setTimeout(resolve, 1000);
            });
        }); // Mock deadline should time out after normal operations.
    });

    afterEach(() => {
        // Restore logger
        console.log = origLog;
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
                    body.Reason.search(expectedErrMessageRegex) !== -1 &&
                    body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
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
                // PhysicalResourceId is undefined for "Create"
                LogicalResourceId: mockLogicalResourceID,
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
        const mockDescribeCustomDomains = sinon.fake(async () => {
            await new Promise((resolve) => setTimeout(resolve, 1000));
        });
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^update record mockDomain: some error \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1 &&
                    body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
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
                LogicalResourceId: mockLogicalResourceID,
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
        const mockDescribeCustomDomains = sinon.fake(async () => {
            await new Promise((resolve) => setTimeout(resolve, 1000));
        });
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^update record mockDomain: wait for record sets change for mockDomain: some error \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1 &&
                    body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
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
                LogicalResourceId: mockLogicalResourceID,
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
                let expectedErrMessageRegex = /^update validation records for domain mockDomain: some error \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1 &&
                    body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
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
                LogicalResourceId: mockLogicalResourceID,
            })
            .expectResolve( err => {
                expect(expectedResponse.isDone()).toBe(true);
                sinon.assert.calledWith(mockDescribeCustomDomains, sinon.match({ ServiceArn: mockServiceARN, }));
            });
    });

    test("fail to find domain information in the service", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockWaitFor = sinon.fake.resolves();
        const mockChangeResourceRecordSets = sinon.fake.resolves({ ChangeInfo: {Id: "mockID", }, });

        // Try to find domain information until all pages are searched through.
        const mockDescribeCustomDomains = sinon.stub();
        mockDescribeCustomDomains.onFirstCall().resolves({
            CustomDomains: [{ DomainName: "some-other-domain", },],
            NextToken: "1",
        });
        mockDescribeCustomDomains.onSecondCall().resolves({
            CustomDomains: [{ DomainName: "some-other-domain", },],
        });

        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets",mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^update validation records for domain mockDomain: domain mockDomain is not associated \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1 &&
                    body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
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
                LogicalResourceId: mockLogicalResourceID,
            })
            .expectResolve( () => {
                expect(expectedResponse.isDone()).toBe(true);

                // Asserts that mockDescribeCustomDomains is called with `NextToken: 1` for at least once;
                // There is no good native way to test individual call arguments: https://github.com/sinonjs/sinon/issues/583.
                sinon.assert.calledWith(mockDescribeCustomDomains, sinon.match({
                    ServiceArn: mockServiceARN,
                    NextToken: "1",
                }));
                sinon.assert.calledTwice(mockDescribeCustomDomains);
            });
    });

    test("fail to wait for app runner to provide validation records", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockWaitFor = sinon.fake.resolves();
        const mockChangeResourceRecordSets = sinon.fake.resolves({ ChangeInfo: {Id: "mockID", }, });
        const mockDescribeCustomDomains = sinon.stub();

        for (let i = 0; i < waitForDomainStatusPendingAttempts; i++) {
            // Mock response such that the domain we are looking for locates at the third page.
            mockDescribeCustomDomains.onCall(i*3).resolves({
                CustomDomains: [{ DomainName: "other-domain", },],
                NextToken: "token",
            });
            mockDescribeCustomDomains.onCall((i*3)+1).resolves({
                CustomDomains: [{ DomainName: "other-domain", },],
                NextToken: "token",
            });
            mockDescribeCustomDomains.onCall((i*3)+2).resolves({
                CustomDomains: [{
                    DomainName: mockCustomDomain,
                    Status: "not-pending",
                },],
                NextToken: "token",
            });
        }
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets",mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^update validation records for domain mockDomain: fail to wait for state pending_certificate_dns_validation, stuck in not-pending \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1 &&
                    body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
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
                LogicalResourceId: mockLogicalResourceID,
            })
            .expectResolve( () => {
                expect(expectedResponse.isDone()).toBe(true);
            });
    });

    test("fail to add cert validation record", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockWaitFor = sinon.fake.resolves();

        const mockDescribeCustomDomains = sinon.stub();
        // Mock response such that the domain we are looking for locates at the third page.
        mockDescribeCustomDomains.onCall(0).resolves({
            CustomDomains: [{
                DomainName: "other-domain",
                CertificateValidationRecords: [
                    {
                        Name: "this shouldn't appear",
                        Value: "this shouldn't appear",
                    },
                ],
            },],
            NextToken: "token",
        });
        mockDescribeCustomDomains.onCall(1).resolves({
            CustomDomains: [{
                DomainName: "other-domain",
            },],
            NextToken: "token",
        });
        mockDescribeCustomDomains.onCall(2).resolves({
            CustomDomains: [{
                DomainName: mockCustomDomain,
                Status: "pending_certificate_dns_validation",
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
            },],
            NextToken: "token",
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
                let expectedErrMessageRegex = /^update validation records for domain mockDomain: update record mock-record-name-2: some error \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1 &&
                    body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
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
                LogicalResourceId: mockLogicalResourceID,
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

    test("fail to wait for domain to become active", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockWaitFor = sinon.fake.resolves();
        const mockDescribeCustomDomains = sinon.stub();
        mockDescribeCustomDomains.onCall(0).resolves({
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
                    Status: "pending_certificate_dns_validation",
                },
            ],
        }); // When we waits for "pending_certificate_dns_validation" so that we can retrieve the validate records to be added.
        const mockChangeResourceRecordSets = sinon.stub();
        mockChangeResourceRecordSets.resolves({ ChangeInfo: {Id: "mockID", }, });

        // Waits for custom domain to become "active".
        for (let i = 0; i < waitForDomainStatusActiveAttempts; i++) {
            mockDescribeCustomDomains.onCall(i + 1).resolves({
                CustomDomains: [
                    {
                        DomainName: mockCustomDomain,
                        Status: "not-active",
                    },
                ],
            });
        }
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
        AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
        AWS.mock("Route53", "waitFor", mockWaitFor);
        AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^fail to wait for domain mockDomain to become active \(Log: .*\)$/;
                return (
                    body.Status === "FAILED" &&
                    body.Reason.search(expectedErrMessageRegex) !== -1 &&
                    body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
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
                LogicalResourceId: mockLogicalResourceID,
            })
            .expectResolve(() => {
                expect(expectedResponse.isDone()).toBe(true);
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
                PhysicalResourceId: mockPhysicalResourceID,
                LogicalResourceId: mockLogicalResourceID,
            })
            .expectReject(() => {});
    });

    test("success", () => {
        const mockTarget = "mockTarget";
        const mockAssociateCustomDomain = sinon.fake.resolves({ DNSTarget: mockTarget, });
        const mockWaitFor = sinon.fake.resolves();
        const mockDescribeCustomDomains = sinon.stub();
        // Successfully wait for custom domain's status to be "pending" after several waits.
        for (let i = 0; i < waitForDomainStatusPendingAttempts - 1; i++) {
            mockDescribeCustomDomains.onCall(i).resolves({
                CustomDomains: [
                    {
                        DomainName: mockCustomDomain,
                        Status: "not-pending",
                    },
                ],
            });
        }
        mockDescribeCustomDomains.onCall(waitForDomainStatusPendingAttempts - 1).resolves({
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
        mockDescribeCustomDomains.onCall(waitForDomainStatusPendingAttempts).resolves({
            CustomDomains: [
                {
                    DomainName: mockCustomDomain,
                    Status: "active",
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
                return body.Status === "SUCCESS"  &&
                       body.PhysicalResourceId === "/associate-domain-app-runner/mockDomain";
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
                PhysicalResourceId: mockPhysicalResourceID,
                LogicalResourceId: mockLogicalResourceID,
            })
            .expectResolve(() => {
                expect(expectedResponse.isDone()).toBe(true);
            });
    });

    test("lambda time out", () => {
        withDeadlineExpired(() => {
            return new Promise(function (resolve, reject) {
                setTimeout(
                    reject,
                    1,
                    new Error("Lambda took longer than 14.5 minutes to add alias")
                );
            });
        });

        const mockAssociateCustomDomain = sinon.fake(async () => {
            await new Promise((resolve) => setTimeout(resolve, 1000));
        });
        AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);

        const expectedResponse = nock(mockResponseURL)
            .put("/", (body) => {
                let expectedErrMessageRegex = /^Lambda took longer than 14.5 minutes to add alias \(Log: .*\)$/;
                return body.Status === "FAILED"  &&
                    body.Reason.search(expectedErrMessageRegex) !== -1 &&
                    body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`;

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
                LogicalResourceId: mockLogicalResourceID,
            })
            .expectResolve(() => {
                expect(expectedResponse.isDone()).toBe(true);
            });
    });
});