// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

/* jshint node: true */
/*jshint esversion: 8 */

const AWS = require("aws-sdk-mock");
const LambdaTester = require("lambda-tester").noVersionCheck();
const {handler, domainStatusPendingVerification, waitForDomainStatusPendingAttempts, waitForDomainStatusActiveAttempts, waitForDomainToBeDisassociatedAttempts, withSleep, reset, withDeadlineExpired} = require("../lib/custom-domain-app-runner");
const sinon = require("sinon");
const nock = require("nock");
let origLog = console.log;

describe("Custom Domain for App Runner Service", () => {
    const [mockServiceARN, mockCustomDomain, mockHostedZoneID, mockResponseURL, mockPhysicalResourceID, mockLogicalResourceID, mockTarget, mockAppDNSName] =
        ["mockService", "mockDomain", "mockHostedZoneID", "https://mock.com/", "mockPhysicalResourceID", "mockLogicalResourceID", "mockTarget", "mockAppDNSName", ];

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

    describe("During CREATE", () => {
        test("unsupported action fails", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^Unsupported request type undefined \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event({
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {},
                    LogicalResourceId: "mockID",
                })
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                });
        });

        test("fail to retrieve hosted zone ID", () => {
            const mockListHostedZonesByName = sinon.fake.rejects(new Error("some error")); // Able to retrieve the hosted zone ID.
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

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
                .event({
                    ResponseURL: mockResponseURL,
                    LogicalResourceId: "mockID",
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                        AppDNSName: mockAppDNSName,
                    },
                })
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                    sinon.assert.calledWith(mockListHostedZonesByName, sinon.match({
                        DNSName: "mockAppDNSName",
                        MaxItems: "1",
                    }));
                });

        });

        test("no hosted zone for app dns domain found", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [],
            }); // Able to retrieve the hosted zone ID.
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

            const request = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^couldn't find any Hosted Zone with DNS name mockAppDNSName \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1
                    );
                })
                .reply(200);
            return LambdaTester(handler)
                .event({
                    ResponseURL: mockResponseURL,
                    LogicalResourceId: "mockID",
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                        AppDNSName: mockAppDNSName,
                    },
                })
                .expectResolve(() => {
                    expect(request.isDone()).toBe(true);
                });
        });

        test("fail to associate custom domain", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockAssociateCustomDomain = sinon.fake.rejects(new Error("some error"));
            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

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
            return LambdaTester(handler)
                .event({
                    RequestType: "Create",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    // PhysicalResourceId is undefined for "Create"
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
                    expect(expectedResponse.isDone()).toBe(true);
                    sinon.assert.calledWith(mockAssociateCustomDomain, sinon.match({
                        DomainName: mockCustomDomain,
                        ServiceArn: mockServiceARN,
                    }));
                });
        });

        test("fail to add the record for custom domain", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockTarget = "mockTarget";
            const mockAssociateCustomDomain = sinon.fake.resolves({DNSTarget: mockTarget,});
            const mockChangeResourceRecordSets = sinon.fake.rejects(new Error("some error"));
            const mockDescribeCustomDomains = sinon.fake(async () => {
                await new Promise(function (resolve, reject) {});
            });
            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

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
            return LambdaTester(handler)
                .event({
                    RequestType: "Create",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
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
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockTarget = "mockTarget";
            const mockAssociateCustomDomain = sinon.fake.resolves({DNSTarget: mockTarget,});
            const mockChangeResourceRecordSets = sinon.fake.resolves({ChangeInfo: {Id: "mockID",},});
            const mockWaitFor = sinon.fake.rejects(new Error("some error"));
            const mockDescribeCustomDomains = sinon.fake(async () => {
                await new Promise(function (resolve, reject) {});
            });
            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

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
            return LambdaTester(handler)
                .event({
                    RequestType: "Create",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(err => {
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
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockAssociateCustomDomain = sinon.fake.resolves({DNSTarget: mockTarget,});
            const mockChangeResourceRecordSets = sinon.fake.resolves({ChangeInfo: {Id: "mockID",},});
            const mockWaitFor = sinon.fake.resolves();
            const mockDescribeCustomDomains = sinon.fake.rejects(new Error("some error"));
            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

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
            return LambdaTester(handler)
                .event({
                    RequestType: "Create",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(err => {
                    expect(expectedResponse.isDone()).toBe(true);
                    sinon.assert.calledWith(mockDescribeCustomDomains, sinon.match({ServiceArn: mockServiceARN,}));
                });
        });

        test("fail to find domain information in the service", () => {
            const mockTarget = "mockTarget";
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockAssociateCustomDomain = sinon.fake.resolves({DNSTarget: mockTarget,});
            const mockWaitFor = sinon.fake.resolves();
            const mockChangeResourceRecordSets = sinon.fake.resolves({ChangeInfo: {Id: "mockID",},});

            // Try to find domain information until all pages are searched through.
            const mockDescribeCustomDomains = sinon.stub();
            mockDescribeCustomDomains.onFirstCall().resolves({
                CustomDomains: [{DomainName: "some-other-domain",},],
                NextToken: "1",
            });
            mockDescribeCustomDomains.onSecondCall().resolves({
                CustomDomains: [{DomainName: "some-other-domain",},],
            });

            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

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
            return LambdaTester(handler)
                .event({
                    RequestType: "Create",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
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
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockAssociateCustomDomain = sinon.fake.resolves({DNSTarget: mockTarget,});
            const mockWaitFor = sinon.fake.resolves();
            const mockChangeResourceRecordSets = sinon.fake.resolves({ChangeInfo: {Id: "mockID",},});
            const mockDescribeCustomDomains = sinon.stub();

            for (let i = 0; i < waitForDomainStatusPendingAttempts; i++) {
                // Mock response such that the domain we are looking for locates at the third page.
                mockDescribeCustomDomains.onCall(i * 3).resolves({
                    CustomDomains: [{DomainName: "other-domain",},],
                    NextToken: "token",
                });
                mockDescribeCustomDomains.onCall((i * 3) + 1).resolves({
                    CustomDomains: [{DomainName: "other-domain",},],
                    NextToken: "token",
                });
                mockDescribeCustomDomains.onCall((i * 3) + 2).resolves({
                    CustomDomains: [{
                        DomainName: mockCustomDomain,
                        Status: "not-pending",
                    },],
                    NextToken: "token",
                });
            }
            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

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
            return LambdaTester(handler)
                .event({
                    RequestType: "Create",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
                    expect(expectedResponse.isDone()).toBe(true);
                });
        });

        test("fail to add cert validation record", () => {
            const mockTarget = "mockTarget";
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockAssociateCustomDomain = sinon.fake.resolves({DNSTarget: mockTarget,});
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
            mockChangeResourceRecordSets.onCall(0).resolves({ChangeInfo: {Id: "mockID",},});
            mockChangeResourceRecordSets.onCall(1).resolves({ChangeInfo: {Id: "mockID",},});
            mockChangeResourceRecordSets.onCall(2).rejects(new Error("some error"));

            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

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
            return LambdaTester(handler)
                .event({
                    RequestType: "Create",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
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
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockAssociateCustomDomain = sinon.fake.rejects(new Error("some error"));
            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
            return LambdaTester(handler)
                .event({
                    ResponseURL: "super weird URL",
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    PhysicalResourceId: mockPhysicalResourceID,
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectReject(() => {
                });
        });

        test("success", () => {
            const mockTarget = "mockTarget";
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockAssociateCustomDomain = sinon.fake.resolves({DNSTarget: mockTarget,});
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
            mockChangeResourceRecordSets.resolves({ChangeInfo: {Id: "mockID",},});

            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

            const expectedResponse = nock(mockResponseURL)
                .put("/", (body) => {
                    return body.Status === "SUCCESS" &&
                        body.PhysicalResourceId === "/associate-domain-app-runner/mockDomain";
                })
                .reply(200);
            return LambdaTester(handler)
                .event({
                    RequestType: "Create",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    PhysicalResourceId: mockPhysicalResourceID,
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
                    expect(expectedResponse.isDone()).toBe(true);
                });
        });

        test("success when domain is already associated", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockAssociateCustomDomain = sinon.fake.rejects("mockDomain is already associated with service");
            const mockDescribeCustomDomains = sinon.stub();
            // In the case where the domain is already associated, we need an additional `DescribeCustomDomains` call
            // to retrieve the `DNSTarget`.
            mockDescribeCustomDomains.onCall(0).resolves({
                DNSTarget: mockTarget,
            });
            mockDescribeCustomDomains.onCall(1).resolves({
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
            mockDescribeCustomDomains.onCall(2).resolves({
                CustomDomains: [
                    {
                        DomainName: mockCustomDomain,
                        Status: "active",
                    },
                ],
            });

            const mockChangeResourceRecordSets = sinon.stub();
            const mockWaitFor = sinon.fake.resolves();
            mockChangeResourceRecordSets.resolves({ChangeInfo: {Id: "mockID",},});

            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

            const expectedResponse = nock(mockResponseURL)
                .put("/", (body) => {
                    return body.Status === "SUCCESS" &&
                        body.PhysicalResourceId === "/associate-domain-app-runner/mockDomain";
                })
                .reply(200);
            return LambdaTester(handler)
                .event({
                    RequestType: "Create",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    PhysicalResourceId: mockPhysicalResourceID,
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
                    expect(expectedResponse.isDone()).toBe(true);
                });
        });

        test("lambda time out", () => {
            withDeadlineExpired(_ => {
                return new Promise(function (resolve, reject) {
                    reject(new Error("lambda time out error"));
                });
            });

            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockAssociateCustomDomain = sinon.fake(async () => {
                await new Promise(function (resolve, reject) {});
            });
            AWS.mock("AppRunner", "associateCustomDomain", mockAssociateCustomDomain);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

            const expectedResponse = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^lambda time out error \(Log: .*\)$/;
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
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
                    expect(expectedResponse.isDone()).toBe(true);
                });
        });
    });

    describe("During DELETE", () => {
        test("fail to disassociate custom domain", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockDisassociateCustomDomain = sinon.fake.rejects(new Error("some error"));
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

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
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    PhysicalResourceId: `/associate-domain-app-runner/mockDomain`,
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve( () => {
                    expect(expectedResponse.isDone()).toBe(true);
                    sinon.assert.calledWith(mockDisassociateCustomDomain, sinon.match({
                        DomainName: mockCustomDomain,
                        ServiceArn: mockServiceARN,
                    }));
                });
        });

        test("do not error out if the domain does not exist", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockDisassociateCustomDomain = sinon.fake.rejects(new Error("No custom domain mockDomain found for the provided service"));
            const mockDescribeCustomDomains = sinon.fake.rejects(new Error("domain mockDomain is not associated"));
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

            const expectedResponse = nock(mockResponseURL)
                .put("/", (body) => {
                    return (
                        body.Status === "SUCCESS" &&
                        body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
                    );
                })
                .reply(200);
            return LambdaTester( handler )
                .event({
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    PhysicalResourceId: `/associate-domain-app-runner/mockDomain`,
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve( () => {
                    expect(expectedResponse.isDone()).toBe(true);
                    sinon.assert.calledWith(mockDisassociateCustomDomain, sinon.match({
                        DomainName: mockCustomDomain,
                        ServiceArn: mockServiceARN,
                    }));
                });
        });

        test("do not error out if a record to be deleted does not exist", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockDisassociateCustomDomain = sinon.fake.resolves({
                DNSTarget: mockTarget,
                CustomDomain: {
                    DomainName: mockCustomDomain,
                    CertificateValidationRecords: [{
                        Name: "mock-record-name-1",
                        Value: "mock-record-value-1",
                    },],
                },
            });
            const mockChangeResourceRecordSets = sinon.fake.rejects(new Error("Tried to delete resource record set [name='mock-record-name-1', type='CNAME'] but it was not found"),);
            const mockDescribeCustomDomains = sinon.fake.rejects(new Error("domain mockDomain is not associated"));
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);

            const expectedResponse = nock(mockResponseURL)
                .put("/", (body) => {
                    return (
                        body.Status === "SUCCESS" &&
                        body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
                    );
                })
                .reply(200);
            return LambdaTester( handler )
                .event({
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    PhysicalResourceId: `/associate-domain-app-runner/mockDomain`,
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve( () => {
                    expect(expectedResponse.isDone()).toBe(true);
                    sinon.assert.calledWith(mockDisassociateCustomDomain, sinon.match({
                        DomainName: mockCustomDomain,
                        ServiceArn: mockServiceARN,
                    }));
                });
        });

        test("fail to remove the custom domain record", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockTarget = "mockTarget";
            const mockDisassociateCustomDomain = sinon.fake.resolves({
                DNSTarget: mockTarget,
                CustomDomain: {
                    CertificateValidationRecords: [
                        {
                            Name: "validate-record-1-name",
                            Value: "validate-record-1-value",
                        },
                    ],
                },
            });
            const mockChangeResourceRecordSets = sinon.stub();
            mockChangeResourceRecordSets.withArgs(
                {
                    ChangeBatch: {
                        Changes: [
                            {
                                Action: "DELETE",
                                ResourceRecordSet: {
                                    Name: "mockDomain",
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
                }
            ).rejects(new Error("some error")); // Rejects for the call for the domain.
            mockChangeResourceRecordSets.resolves(); // Resolves for other calls.

            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);

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
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve( () => {
                    expect(expectedResponse.isDone()).toBe(true);
                    sinon.assert.calledOnce(mockDisassociateCustomDomain);
                    sinon.assert.calledWith(mockChangeResourceRecordSets, sinon.match({
                        ChangeBatch: {
                            Changes: [
                                {
                                    Action: "DELETE",
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
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockTarget = "mockTarget";
            const mockDisassociateCustomDomain = sinon.fake.resolves({
                DNSTarget: mockTarget,
                CustomDomain: {
                    CertificateValidationRecords: [
                        {
                            Name: "validate-record-1-name",
                            Value: "validate-record-1-value",
                        },
                    ],
                },
            });
            const mockChangeResourceRecordSets = sinon.stub();
            mockChangeResourceRecordSets.withArgs(
                {
                    ChangeBatch: {
                        Changes: [
                            {
                                Action: "DELETE",
                                ResourceRecordSet: {
                                    Name: "mockDomain",
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
                }
            ).resolves({ ChangeInfo: {Id: "mockDomainID", }, }); // Resolves with "mockDomainID" for the call for the domain.
            mockChangeResourceRecordSets.resolves({ ChangeInfo: {Id: "mockValidationRecordID", }, }); // Resolves "mockValidationRecordID" for other calls.
            const mockWaitFor = sinon.stub();
            mockWaitFor.withArgs('resourceRecordSetsChanged', {
                $waiter: {
                    delay: 30,
                    maxAttempts: 10,
                },
                Id: "mockDomainID",
            }).rejects(new Error("some error")); // Rejects for the call that update the domain record.
            mockWaitFor.withArgs('resourceRecordSetsChanged', {
                $waiter: {
                    delay: 30,
                    maxAttempts: 10,
                },
                Id: "mockValidationRecordID",
            }).resolves(); // Resolves for the other calls.

            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);

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
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve( err => {
                    expect(expectedResponse.isDone()).toBe(true);
                    sinon.assert.calledOnce(mockDisassociateCustomDomain);
                    sinon.assert.calledWith(mockChangeResourceRecordSets, sinon.match({
                        ChangeBatch: {
                            Changes: [
                                {
                                    Action: "DELETE",
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

        test("fail to remove cert validation records", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockDisassociateCustomDomain = sinon.fake.resolves({
                DNSTarget: mockTarget,
                CustomDomain: {
                    DomainName: mockCustomDomain,
                    CertificateValidationRecords: [
                        {
                            Name: "mock-record-name-1",
                            Value: "mock-record-value-1",
                        },
                    ],
                },
            });
            const mockChangeResourceRecordSets = sinon.stub();
            mockChangeResourceRecordSets.withArgs(
                {
                    ChangeBatch: {
                        Changes: [
                            {
                                Action: "DELETE",
                                ResourceRecordSet: {
                                    Name: "mockDomain",
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
                }
            ).resolves({ ChangeInfo: {Id: "mockDomainID", }, }); // Resolves with "mockDomainID" for the call for the domain.
            mockChangeResourceRecordSets.withArgs({
                ChangeBatch: {
                    Changes: [
                        {
                            Action: "DELETE",
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
            }).rejects(new Error("some error")); // Rejects the other calls.
            const mockWaitFor = sinon.fake.resolves();

            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);

            const expectedResponse = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^delete validation records for domain mockDomain: update record mock-record-name-1: some error \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1 &&
                        body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
                    );
                })
                .reply(200);
            return LambdaTester( handler )
                .event({
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve( () => {
                    expect(expectedResponse.isDone()).toBe(true);
                    sinon.assert.calledWith(mockChangeResourceRecordSets, sinon.match({
                        ChangeBatch: {
                            Changes: [
                                {
                                    Action: "DELETE",
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
                });
        });

        test("fail to wait for domain to be disassociated", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockDisassociateCustomDomain = sinon.fake.resolves({
                DNSTarget: mockTarget,
                CustomDomain: {
                    DomainName: mockCustomDomain,
                    CertificateValidationRecords: [{
                        Name: "mock-record-name-1",
                        Value: "mock-record-value-1",
                    }, ],
                },
            });
            const mockChangeResourceRecordSets = sinon.stub().resolves({ ChangeInfo: {Id: "mockChangeID", }, });
            const mockWaitFor = sinon.fake.resolves();

            // Attempts to wait for custom domain to become disassociated max out.
            const mockDescribeCustomDomains = sinon.stub();
            for (let i = 0; i < waitForDomainToBeDisassociatedAttempts; i++) {
                mockDescribeCustomDomains.onCall(i).resolves({
                    CustomDomains: [
                        {
                            DomainName: mockCustomDomain,
                            Status: "deleting",
                        },
                    ],
                });
            }
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

            const expectedResponse = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^fail to wait for domain mockDomain to be disassociated \(Log: .*\)$/;
                    return (
                        body.Status === "FAILED" &&
                        body.Reason.search(expectedErrMessageRegex) !== -1 &&
                        body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`
                    );
                })
                .reply(200);
            return LambdaTester( handler )
                .event({
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
                    expect(expectedResponse.isDone()).toBe(true);
                });
        });

        test("fail to delete domain", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockDisassociateCustomDomain = sinon.fake.resolves({
                DNSTarget: mockTarget,
                CustomDomain: {
                    DomainName: mockCustomDomain,
                    CertificateValidationRecords: [{
                        Name: "mock-record-name-1",
                        Value: "mock-record-value-1",
                    }],
                },
            });
            const mockChangeResourceRecordSets = sinon.stub().resolves({ ChangeInfo: {Id: "mockChangeID", }, });
            const mockWaitFor = sinon.fake.resolves();

            // Domain status becomes delete_failed at the last attempt;
            const mockDescribeCustomDomains = sinon.stub();
            for (let i = 0; i < waitForDomainToBeDisassociatedAttempts - 1; i++) {
                mockDescribeCustomDomains.onCall(i).resolves({
                    CustomDomains: [
                        {
                            DomainName: mockCustomDomain,
                            Status: "deleting",
                        },
                    ],
                });
            }
            mockDescribeCustomDomains.onCall(waitForDomainToBeDisassociatedAttempts - 1).resolves({
                CustomDomains: [
                    {
                        DomainName: mockCustomDomain,
                        Status: "delete_failed",
                    },
                ],
            });
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);
            AWS.mock("Route53", "changeResourceRecordSets", mockChangeResourceRecordSets);
            AWS.mock("Route53", "waitFor", mockWaitFor);
            AWS.mock("AppRunner", "describeCustomDomains", mockDescribeCustomDomains);

            const expectedResponse = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^fail to disassociate domain mockDomain: domain status is delete_failed \(Log: .*\)$/;
                    return body.Status === "FAILED"  &&
                        body.Reason.search(expectedErrMessageRegex) !== -1 &&
                        body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`;

                })
                .reply(200);
            return LambdaTester( handler )
                .event({
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
                    expect(expectedResponse.isDone()).toBe(true);
                });
        });

        test("success", () => {
            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockDisassociateCustomDomain = sinon.fake.resolves({
                DNSTarget: mockTarget,
                CustomDomain: {
                    DomainName: mockCustomDomain,
                    CertificateValidationRecords: [{
                        Name: "mock-record-name-1",
                        Value: "mock-record-value-1",
                    },],
                },
            });
            const mockChangeResourceRecordSets = sinon.stub().resolves({ ChangeInfo: {Id: "mockChangeID", }, });
            const mockWaitFor = sinon.fake.resolves();

            // Domain is successfully disassociated at the last attempt.
            const mockDescribeCustomDomains = sinon.stub();
            for (let i = 0; i < waitForDomainToBeDisassociatedAttempts - 1; i++) {
                mockDescribeCustomDomains.onCall(i).resolves({
                    CustomDomains: [
                        {
                            DomainName: mockCustomDomain,
                            Status: "deleting",
                        },
                        {
                            DomainName: "other-domain",
                            Status: "active",
                        },
                    ],
                });
            }
            mockDescribeCustomDomains.onCall(waitForDomainToBeDisassociatedAttempts - 1).resolves({
                CustomDomains: [
                    {
                        DomainName: "other-domain",
                        Status: "active",
                    },
                ],
            });
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);
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
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    PhysicalResourceId: mockPhysicalResourceID,
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
                    expect(expectedResponse.isDone()).toBe(true);
                });
        });

        test("lambda time out", () => {
            withDeadlineExpired(_ => {
                return new Promise(function (_, reject) {
                    reject(new Error("lambda time out error"));
                });
            });

            const mockListHostedZonesByName = sinon.fake.resolves({
                HostedZones: [
                    {
                        Id: "/hostedzone/mockHostedZoneID",
                    },
                ],
            }); // Able to retrieve the hosted zone ID.
            const mockDisassociateCustomDomain = sinon.fake(async () => {
                await new Promise(_ => {});
            });
            AWS.mock("Route53", "listHostedZonesByName", mockListHostedZonesByName);
            AWS.mock("AppRunner", "disassociateCustomDomain", mockDisassociateCustomDomain);

            const expectedResponse = nock(mockResponseURL)
                .put("/", (body) => {
                    let expectedErrMessageRegex = /^lambda time out error \(Log: .*\)$/;
                    return body.Status === "FAILED"  &&
                        body.Reason.search(expectedErrMessageRegex) !== -1 &&
                        body.PhysicalResourceId === `/associate-domain-app-runner/mockDomain`;

                })
                .reply(200);
            return LambdaTester( handler )
                .event({
                    RequestType: "Delete",
                    ResponseURL: mockResponseURL,
                    ResourceProperties: {
                        ServiceARN: mockServiceARN,
                        AppDNSRole: "",
                        CustomDomain: mockCustomDomain,
                    },
                    LogicalResourceId: mockLogicalResourceID,
                })
                .expectResolve(() => {
                    expect(expectedResponse.isDone()).toBe(true);
                });
        });
    });
});
