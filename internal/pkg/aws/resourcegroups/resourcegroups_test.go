// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package resourcegroups

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var testTags = map[string]string{
	"copilot-environment": "test",
}

const (
	testResourceType    = "AWS::CloudWatch::Alarm"
	testTagsQueryString = `{"ResourceTypeFilters":["AWS::CloudWatch::Alarm"],"TagFilters":[{"Key":"copilot-environment","Values":["test"]}]}` // NOTE only using one tag, since ranging over a map is not idempotent

	testArn  = "arn:aws:cloudwatch:us-west-2:1234567890:alarm:SDc-ReadCapacityUnitsLimit-BasicAlarm"
	mockArn1 = "arn:aws:cloudwatch:us-west-2:1234567890:alarm:mockAlarmName1"
	mockArn2 = "arn:aws:cloudwatch:us-west-2:1234567890:alarm:mockAlarmName2"
)

func TestResourceGroups_SearchResourcesQuery(t *testing.T) {
	testCases := map[string]struct {
		inTags         map[string]string
		inResourceType string
		expectedQuery  string
		expectedErr    error
	}{
		"returns query string": {
			inTags:         testTags,
			inResourceType: testResourceType,
			expectedQuery:  testTagsQueryString,
			expectedErr:    nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockapi(ctrl)
			rg := &ResourceGroups{client: mockClient}

			// WHEN
			actualQuery, actualErr := rg.searchResourcesQuery(tc.inResourceType, tc.inTags)

			// THEN
			if actualErr != nil {
				require.EqualError(t, tc.expectedErr, actualErr.Error())
			} else {
				require.Equal(t, tc.expectedQuery, actualQuery)
			}
		})
	}
}

func TestResourceGroups_GetResourcesByTags(t *testing.T) {
	mockRequest := &resourcegroups.SearchResourcesInput{
		NextToken: nil,
		ResourceQuery: &resourcegroups.ResourceQuery{
			Type:  aws.String(resourceQueryType),
			Query: aws.String(testTagsQueryString),
		},
	}
	mockResponse := &resourcegroups.SearchResourcesOutput{
		ResourceIdentifiers: []*resourcegroups.ResourceIdentifier{
			{
				ResourceType: aws.String(testResourceType),
				ResourceArn:  aws.String(testArn),
			},
		},
	}
	mockError := errors.New("some error")

	testCases := map[string]struct {
		inTags         map[string]string
		inResourceType string
		setupMocks     func(m *mocks.Mockapi)
		expectedOut    []string
		expectedErr    error
	}{
		"returns list of arns": {
			inTags:         testTags,
			inResourceType: testResourceType,
			setupMocks: func(m *mocks.Mockapi) {
				m.EXPECT().SearchResources(mockRequest).Return(mockResponse, nil)
			},
			expectedOut: []string{testArn},
			expectedErr: nil,
		},
		"wraps error from API call": {
			inTags:         testTags,
			inResourceType: testResourceType,
			setupMocks: func(m *mocks.Mockapi) {
				m.EXPECT().SearchResources(mockRequest).Return(nil, mockError)
			},
			expectedOut: nil,
			expectedErr: fmt.Errorf("search resource group with resource type %s: %w", testResourceType, mockError),
		},
		"success with pagination": {
			inTags:         testTags,
			inResourceType: testResourceType,
			setupMocks: func(m *mocks.Mockapi) {
				gomock.InOrder(
					m.EXPECT().SearchResources(&resourcegroups.SearchResourcesInput{
						NextToken: nil,
						ResourceQuery: &resourcegroups.ResourceQuery{
							Type:  aws.String(resourceQueryType),
							Query: aws.String(testTagsQueryString),
						},
					}).Return(&resourcegroups.SearchResourcesOutput{
						NextToken: aws.String("mockNextToken"),
						ResourceIdentifiers: []*resourcegroups.ResourceIdentifier{
							{
								ResourceArn: aws.String(mockArn1),
							},
						},
					}, nil),
					m.EXPECT().SearchResources(&resourcegroups.SearchResourcesInput{
						NextToken: aws.String("mockNextToken"),
						ResourceQuery: &resourcegroups.ResourceQuery{
							Type:  aws.String(resourceQueryType),
							Query: aws.String(testTagsQueryString),
						},
					}).Return(&resourcegroups.SearchResourcesOutput{
						NextToken: nil,
						ResourceIdentifiers: []*resourcegroups.ResourceIdentifier{
							{
								ResourceArn: aws.String(mockArn2),
							},
						},
					}, nil),
				)
			},
			expectedOut: []string{mockArn1, mockArn2},
			expectedErr: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockapi(ctrl)
			rg := &ResourceGroups{client: mockClient}

			// WHEN
			tc.setupMocks(mockClient)
			actualOut, actualErr := rg.GetResourcesByTags(tc.inResourceType, tc.inTags)

			// THEN
			if actualErr != nil {
				require.EqualError(t, tc.expectedErr, actualErr.Error())
			} else {
				require.Equal(t, tc.expectedOut, actualOut)
			}
		})
	}
}
