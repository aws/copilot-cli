// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
'use strict';

describe('Desired count delegation Handler', () => {
  const AWS = require('aws-sdk-mock');
  const sinon = require('sinon');
  const DesiredCountDelegation = require('../lib/desired-count-delegation');
  const LambdaTester = require('lambda-tester').noVersionCheck();
  const nock = require('nock');
  const responseURL = 'https://cloudwatch-response-mock.example.com/';
  const testRequestId = 'f4ef1b10-c39a-44e3-99c0-fbf7e53c3943';
  let origLog = console.log;

  const testCluster = 'mockClusterName';
  const testFamily = 'mockFamilyName';
  const testNextToken = 'mockNextToken';

  beforeEach(() => {
    DesiredCountDelegation.withDefaultResponseURL(responseURL);
    // Prevent logging.
    console.log = function() { };
  });
  afterEach(() => {
    // Restore logger
    AWS.restore();
    console.log = origLog;
  });

  test('update operation', () => {
    const listTasksFake = sinon.fake.resolves({
      taskArns: [
        "mockTask1",
        "mockTask2",
      ]
    });
    AWS.mock('ECS', 'listTasks', listTasksFake);
    const request = nock(responseURL).put('/', body => {
      return body.Status === 'SUCCESS' && body.Data.DesiredCount == 2;
    }).reply(200);

    return LambdaTester(DesiredCountDelegation.desiredCountHandler)
      .event({
        RequestType: 'Update',
        RequestId: testRequestId,
        ResponseURL: responseURL,
        ResourceProperties: {
          Cluster: testCluster,
          Family: testFamily,
          DesiredCount: 3,
        }
      }).expectResolve(() => {
      sinon.assert.calledWith(listTasksFake, sinon.match({
        cluster: testCluster,
        family: testFamily
      }));
      expect(request.isDone()).toBe(true);
    });
  });

  test('update operation with pagination', () => {
    const listTasksFake = sinon.stub();
    listTasksFake.onCall(0).resolves({
      taskArns: [
        "mockTask1",
        "mockTask2",
      ],
      nextToken: testNextToken
    });
    listTasksFake.onCall(1).resolves({
      taskArns: [
        "mockTask3",
        "mockTask4",
      ],
    });
    AWS.mock('ECS', 'listTasks', listTasksFake);
    const request = nock(responseURL).put('/', body => {
      return body.Status === 'SUCCESS' && body.Data.DesiredCount == 4;
    }).reply(200);

    return LambdaTester(DesiredCountDelegation.desiredCountHandler)
      .event({
        RequestType: 'Update',
        RequestId: testRequestId,
        ResponseURL: responseURL,
        ResourceProperties: {
          Cluster: testCluster,
          Family: testFamily,
          DesiredCount: 3,
        }
      }).expectResolve(() => {
      sinon.assert.calledWith(listTasksFake.firstCall, sinon.match({
        cluster: testCluster,
        family: testFamily
      }));
      sinon.assert.calledWith(listTasksFake.secondCall, sinon.match({
        cluster: testCluster,
        family: testFamily,
        nextToken: testNextToken
      }));
      expect(request.isDone()).toBe(true);
    });
  });

  test('create operation', () => {
      const mockListTasks = sinon.stub();

      AWS.mock('ECS', 'listTasks', mockListTasks)
      const request = nock(responseURL).put('/', body => {
          return body.Status === 'SUCCESS' && body.Data.DesiredCount == 3;
      }).reply(200);

      return LambdaTester(DesiredCountDelegation.desiredCountHandler)
          .event({
              RequestType: 'Create',
              RequestId: testRequestId,
              ResponseURL: responseURL,
              ResourceProperties: {
                Cluster: testCluster,
                Family: testFamily,
                DesiredCount: 3,
              }
          }).expectResolve(() => {
          sinon.assert.notCalled(mockListTasks);
          expect(request.isDone()).toBe(true);
      });
  });

  test('delete operation should do nothing', () => {
      const mockListTasks = sinon.stub();

      AWS.mock('ECS', 'listTasks', mockListTasks)
      const request = nock(responseURL).put('/', body => {
          return body.Status === 'SUCCESS';
      }).reply(200);

      return LambdaTester(DesiredCountDelegation.desiredCountHandler)
          .event({
              RequestType: 'Delete',
              ResponseURL: responseURL,
          }).expectResolve(() => {
          sinon.assert.notCalled(mockListTasks);
          expect(request.isDone()).toBe(true);
      });
  });
});