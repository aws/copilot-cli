// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

describe("DNS Delegation Handler", () => {
  const r53 = require("@aws-sdk/client-route-53");
  const{mockClient} = require("aws-sdk-client-mock");
  const LambdaTester = require("lambda-tester").noVersionCheck();
  const sinon = require("sinon");
  const dnsDelegationHandler = require("../lib/dns-delegation");
  const nock = require("nock");
  const ResponseURL = "https://cloudwatch-response-mock.example.com/";
  const LogGroup = "/aws/lambda/testLambda";
  const LogStream = "2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd";

  let origLog = console.log;

  const testRequestId = "f4ef1b10-c39a-44e3-99c0-fbf7e53c3943";
  const testDomainName = "example.com";
  const testSubDomainName = "test.example.com";
  const testNameServers = ["ns1.com"];
  const testIAMRole = "arn:aws:iam::00000000000:role/DNSDelegationRole";
  const r53Mock = mockClient(r53.Route53Client);
  beforeEach(() => {
    dnsDelegationHandler.withDefaultResponseURL(ResponseURL);
    dnsDelegationHandler.withDefaultLogGroup(LogGroup);
    dnsDelegationHandler.withDefaultLogStream(LogStream);
    console.log = function () {};
    dnsDelegationHandler.waitForRecordSetChange = function () { 
      return Promise.resolve({});
    };
  });
  afterEach(() => {
    r53Mock.reset();
    console.log = origLog;
  });

  test("Empty event payload fails", () => {
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return (
          body.Status === "FAILED" &&
          body.Reason ===
            "Unsupported request type undefined (Log: /aws/lambda/testLambda/2021/06/28/[$LATEST]9b93a7dca7344adeb193d15c092dbbfd)"
        );
      })
      .reply(200);
    return LambdaTester(dnsDelegationHandler.domainDelegationHandler)
      .event({
        RequestId: testRequestId,
        ResourceProperties: {
          DomainName: testDomainName,
          SubdomainName: testSubDomainName,
          NameServers: testNameServers,
          RootDNSRole: testIAMRole,
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
    return LambdaTester(dnsDelegationHandler.domainDelegationHandler)
      .event({
        RequestType: bogusType,
        RequestId: testRequestId,
        ResourceProperties: {
          DomainName: testDomainName,
          SubdomainName: testSubDomainName,
          NameServers: testNameServers,
          RootDNSRole: testIAMRole,
        },
      })
      .expectResolve(() => {
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation creates a subdomain", () => {
    const fakeHostedZone = "/hostedzone/1234455";
    const fakeHostedZoneId = "1234455";
    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [
        {
          Id: fakeHostedZone,
        },
      ],
    });

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    r53Mock.on(r53.ListHostedZonesByNameCommand).callsFake(listHostedZonesByNameFake);
    r53Mock.on(r53.ChangeResourceRecordSetsCommand).callsFake(changeResourceRecordSetsFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(dnsDelegationHandler.domainDelegationHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          DomainName: testDomainName,
          SubdomainName: testSubDomainName,
          NameServers: testNameServers,
          RootDNSRole: testIAMRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: testDomainName,
          })
        );
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: {
              Changes: [
                {
                  Action: "UPSERT",
                  ResourceRecordSet: {
                    Name: testSubDomainName,
                    Type: "NS",
                    TTL: 60,
                    ResourceRecords: [
                      {
                        Value: testNameServers[0],
                      },
                    ],
                  },
                },
              ],
            },
            HostedZoneId: fakeHostedZoneId,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Create operation fails if there is no domain hostedzone", () => {
    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [],
    });

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    r53Mock.on(r53.ListHostedZonesByNameCommand).callsFake(listHostedZonesByNameFake);
    r53Mock.on(r53.ChangeResourceRecordSetsCommand).callsFake(changeResourceRecordSetsFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(dnsDelegationHandler.domainDelegationHandler)
      .event({
        RequestType: "Create",
        RequestId: testRequestId,
        ResourceProperties: {
          DomainName: testDomainName,
          SubdomainName: testSubDomainName,
          NameServers: testNameServers,
          RootDNSRole: testIAMRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: testDomainName,
          })
        );
        sinon.assert.notCalled(changeResourceRecordSetsFake);
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation removes a subdomain", () => {
    const fakeHostedZone = "/hostedzone/1234455";
    const fakeHostedZoneId = "1234455";
    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [
        {
          Id: fakeHostedZone,
        },
      ],
    });

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    const listResourceRecordSetsFake = sinon.fake.resolves({
      ResourceRecordSets: [
        {
          Name: `${testSubDomainName}.`,
          Type: "NS",
          ResourceRecords: [
            {
              Value: testNameServers[0],
            },
          ],
        },
      ],
    });

    r53Mock.on(r53.ListHostedZonesByNameCommand).callsFake(listHostedZonesByNameFake);
    r53Mock.on(r53.ListResourceRecordSetsCommand).callsFake(listResourceRecordSetsFake);
    r53Mock.on(r53.ChangeResourceRecordSetsCommand).callsFake(changeResourceRecordSetsFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "SUCCESS";
      })
      .reply(200);

    return LambdaTester(dnsDelegationHandler.domainDelegationHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {
          DomainName: testDomainName,
          SubdomainName: testSubDomainName,
          RootDNSRole: testIAMRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: testDomainName,
          })
        );
        sinon.assert.calledWith(
          listResourceRecordSetsFake,
          sinon.match({
            HostedZoneId: fakeHostedZoneId,
            MaxItems: "1",
            StartRecordType: "NS",
            StartRecordName: testSubDomainName,
          })
        );
        sinon.assert.calledWith(
          changeResourceRecordSetsFake,
          sinon.match({
            ChangeBatch: {
              Changes: [
                {
                  Action: "DELETE",
                  ResourceRecordSet: {
                    Name: testSubDomainName,
                    Type: "NS",
                    TTL: 60,
                    ResourceRecords: [
                      {
                        Value: testNameServers[0],
                      },
                    ],
                  },
                },
              ],
            },
            HostedZoneId: fakeHostedZoneId,
          })
        );
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation fails if there is no domain hostedzone", () => {
    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [],
    });

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    r53Mock.on(r53.ListHostedZonesByNameCommand).callsFake(listHostedZonesByNameFake);
    r53Mock.on(r53.ChangeResourceRecordSetsCommand).callsFake(changeResourceRecordSetsFake);

    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(dnsDelegationHandler.domainDelegationHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {
          DomainName: testDomainName,
          SubdomainName: testSubDomainName,
          RootDNSRole: testIAMRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: testDomainName,
          })
        );
        sinon.assert.notCalled(changeResourceRecordSetsFake);
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation fails if subdomain record is not there", () => {
    const fakeHostedZone = "/hostedzone/1234455";
    const fakeHostedZoneId = "1234455";
    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [
        {
          Id: fakeHostedZone,
        },
      ],
    });

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    const listResourceRecordSetsFake = sinon.fake.resolves({
      ResourceRecordSets: [
        {
          Name: `${testDomainName}.`,
          Type: "NS",
          ResourceRecords: [
            {
              Value: testNameServers[0],
            },
          ],
        },
      ],
    });

    r53Mock.on(r53.ListHostedZonesByNameCommand).callsFake(listHostedZonesByNameFake);
    r53Mock.on(r53.ListResourceRecordSetsCommand).callsFake(listResourceRecordSetsFake);
    r53Mock.on(r53.ChangeResourceRecordSetsCommand).callsFake(changeResourceRecordSetsFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(dnsDelegationHandler.domainDelegationHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {
          DomainName: testDomainName,
          SubdomainName: testSubDomainName,
          RootDNSRole: testIAMRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: testDomainName,
          })
        );
        sinon.assert.calledWith(
          listResourceRecordSetsFake,
          sinon.match({
            HostedZoneId: fakeHostedZoneId,
            MaxItems: "1",
            StartRecordType: "NS",
            StartRecordName: testSubDomainName,
          })
        );
        sinon.assert.notCalled(changeResourceRecordSetsFake);
        expect(request.isDone()).toBe(true);
      });
  });

  test("Delete operation fails if subdomain record type is not NS", () => {
    const fakeHostedZone = "/hostedzone/1234455";
    const fakeHostedZoneId = "1234455";
    const listHostedZonesByNameFake = sinon.fake.resolves({
      HostedZones: [
        {
          Id: fakeHostedZone,
        },
      ],
    });

    const changeResourceRecordSetsFake = sinon.fake.resolves({
      ChangeInfo: {
        Id: "bogus",
      },
    });

    const listResourceRecordSetsFake = sinon.fake.resolves({
      ResourceRecordSets: [
        {
          Name: `${testSubDomainName}.`,
          Type: "SOA",
          ResourceRecords: [
            {
              Value: testNameServers[0],
            },
          ],
        },
      ],
    });

    r53Mock.on(r53.ListHostedZonesByNameCommand).callsFake(listHostedZonesByNameFake);
    r53Mock.on(r53.ListResourceRecordSetsCommand).callsFake(listResourceRecordSetsFake);
    r53Mock.on(r53.ChangeResourceRecordSetsCommand).callsFake(changeResourceRecordSetsFake);
    const request = nock(ResponseURL)
      .put("/", (body) => {
        return body.Status === "FAILED";
      })
      .reply(200);

    return LambdaTester(dnsDelegationHandler.domainDelegationHandler)
      .event({
        RequestType: "Delete",
        RequestId: testRequestId,
        ResourceProperties: {
          DomainName: testDomainName,
          SubdomainName: testSubDomainName,
          RootDNSRole: testIAMRole,
        },
      })
      .expectResolve(() => {
        sinon.assert.calledWith(
          listHostedZonesByNameFake,
          sinon.match({
            DNSName: testDomainName,
          })
        );
        sinon.assert.calledWith(
          listResourceRecordSetsFake,
          sinon.match({
            HostedZoneId: fakeHostedZoneId,
            MaxItems: "1",
            StartRecordType: "NS",
            StartRecordName: testSubDomainName,
          })
        );
        sinon.assert.notCalled(changeResourceRecordSetsFake);
        expect(request.isDone()).toBe(true);
      });
  });
});
