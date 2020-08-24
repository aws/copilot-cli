// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package resourcegroups

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	rgapi "github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var testTags = map[string]string{
	"copilot-environment": "test",
}

const (
	testResourceType = "cloudwatch:alarm"

	testArn  = "arn:aws:cloudwatch:us-west-2:1234567890:alarm:SDc-ReadCapacityUnitsLimit-BasicAlarm"
	mockArn1 = "arn:aws:cloudwatch:us-west-2:1234567890:alarm:mockAlarmName1"
	mockArn2 = "arn:aws:cloudwatch:us-west-2:1234567890:alarm:mockAlarmName2"
)

func TestResourceGroups_GetResourcesByTags(t *testing.T) {
	mockRequest := &rgapi.GetResourcesInput{
		PaginationToken:     nil,
		ResourceTypeFilters: aws.StringSlice([]string{testResourceType}),
		TagFilters: []*rgapi.TagFilter{
			{
				Key:    aws.String("copilot-environment"),
				Values: aws.StringSlice([]string{"test"}),
			},
		},
	}
	mockResponse := &rgapi.GetResourcesOutput{
		ResourceTagMappingList: []*rgapi.ResourceTagMapping{
			{
				ResourceARN: aws.String(testArn),
				Tags: []*rgapi.Tag{
					{
						Key:   aws.String("copilot-environment"),
						Value: aws.String("test"),
					},
				},
			},
		},
	}
	mockError := errors.New("some error")

	testCases := map[string]struct {
		inTags         map[string]string
		inResourceType string
		setupMocks     func(m *mocks.Mockapi)
		expectedOut    []*Resource
		expectedErr    error
	}{
		"returns list of arns": {
			inTags:         testTags,
			inResourceType: testResourceType,
			setupMocks: func(m *mocks.Mockapi) {
				m.EXPECT().GetResources(mockRequest).Return(mockResponse, nil)
			},
			expectedOut: []*Resource{
				{
					ARN:  testArn,
					Tags: testTags,
				},
			},
			expectedErr: nil,
		},
		"wraps error from API call": {
			inTags:         testTags,
			inResourceType: testResourceType,
			setupMocks: func(m *mocks.Mockapi) {
				m.EXPECT().GetResources(mockRequest).Return(nil, mockError)
			},
			expectedOut: nil,
			expectedErr: fmt.Errorf("get resource: some error"),
		},
		"success with pagination": {
			inTags:         testTags,
			inResourceType: testResourceType,
			setupMocks: func(m *mocks.Mockapi) {
				gomock.InOrder(
					m.EXPECT().GetResources(mockRequest).Return(&rgapi.GetResourcesOutput{
						PaginationToken: aws.String("mockNextToken"),
						ResourceTagMappingList: []*rgapi.ResourceTagMapping{
							{
								ResourceARN: aws.String(mockArn1),
								Tags:        []*rgapi.Tag{{Key: aws.String("copilot-environment"), Value: aws.String("test")}},
							},
						},
					}, nil),
					m.EXPECT().GetResources(&rgapi.GetResourcesInput{
						PaginationToken:     aws.String("mockNextToken"),
						ResourceTypeFilters: aws.StringSlice([]string{testResourceType}),
						TagFilters: []*rgapi.TagFilter{
							{
								Key:    aws.String("copilot-environment"),
								Values: aws.StringSlice([]string{"test"}),
							},
						},
					}).Return(&rgapi.GetResourcesOutput{
						PaginationToken: nil,
						ResourceTagMappingList: []*rgapi.ResourceTagMapping{
							{
								ResourceARN: aws.String(mockArn2),
								Tags:        []*rgapi.Tag{{Key: aws.String("copilot-environment"), Value: aws.String("test")}},
							},
						},
					}, nil),
				)
			},
			expectedOut: []*Resource{
				{
					ARN:  mockArn1,
					Tags: testTags,
				},
				{
					ARN:  mockArn2,
					Tags: testTags,
				},
			},
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
