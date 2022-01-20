// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	sdkcfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type stackDescriberMocks struct {
	cfn *mocks.Mockcfn
}

func TestStackDescriber_Describe(t *testing.T) {
	const mockStackName = "phonetool"
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks stackDescriberMocks)

		wantedDescription StackDescription
		wantedError       error
	}{
		"return error if fail to describe stack": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.cfn.EXPECT().Describe(mockStackName).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("describe stack phonetool: some error"),
		},
		"success": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.cfn.EXPECT().Describe(mockStackName).Return(&cloudformation.StackDescription{
						Parameters: []*sdkcfn.Parameter{
							{
								ParameterKey:   aws.String("mockParamKey"),
								ParameterValue: aws.String("mockParamVal"),
							},
						},
						Outputs: []*sdkcfn.Output{
							{
								OutputKey:   aws.String("mockOutputKey"),
								OutputValue: aws.String("mockOutputVal"),
							},
						},
						Tags: []*sdkcfn.Tag{
							{
								Key:   aws.String("mockTagKey"),
								Value: aws.String("mockTagVal"),
							},
						},
					}, nil),
				)
			},
			wantedDescription: StackDescription{
				Parameters: map[string]string{"mockParamKey": "mockParamVal"},
				Tags:       map[string]string{"mockTagKey": "mockTagVal"},
				Outputs:    map[string]string{"mockOutputKey": "mockOutputVal"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockcfn := mocks.NewMockcfn(ctrl)
			mocks := stackDescriberMocks{
				cfn: mockcfn,
			}

			tc.setupMocks(mocks)

			d := &StackDescriber{
				name: mockStackName,
				cfn:  mockcfn,
			}

			// WHEN
			actual, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedDescription, actual)
			}
		})
	}
}

func TestStackDescriber_Resources(t *testing.T) {
	const mockStackName = "phonetool"
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks stackDescriberMocks)

		wantedResources []*Resource
		wantedError     error
	}{
		"return error if fail to get stack resources": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.cfn.EXPECT().StackResources(mockStackName).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve resources for stack phonetool: some error"),
		},
		"success": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.cfn.EXPECT().StackResources(mockStackName).Return([]*cloudformation.StackResource{
						{
							ResourceType:       aws.String("mockResourceType"),
							PhysicalResourceId: aws.String("mockPhysicalID"),
							LogicalResourceId:  aws.String("mockLogicalID"),
						},
					}, nil),
				)
			},
			wantedResources: []*Resource{
				{
					Type:       "mockResourceType",
					PhysicalID: "mockPhysicalID",
					LogicalID:  "mockLogicalID",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockcfn := mocks.NewMockcfn(ctrl)
			mocks := stackDescriberMocks{
				cfn: mockcfn,
			}

			tc.setupMocks(mocks)

			d := &StackDescriber{
				name: mockStackName,
				cfn:  mockcfn,
			}

			// WHEN
			actual, err := d.Resources()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedResources, actual)
			}
		})
	}
}

func TestStackDescriber_Metadata(t *testing.T) {
	const mockStackName = "phonetool"
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks stackDescriberMocks)

		wantedMetadata string
		wantedError    error
	}{
		"return error if fail to get stack metadata": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.cfn.EXPECT().Metadata(gomock.Any()).Return("", mockErr),
				)
			},
			wantedError: fmt.Errorf("get metadata for stack phonetool: some error"),
		},
		"success": {
			setupMocks: func(m stackDescriberMocks) {
				gomock.InOrder(
					m.cfn.EXPECT().Metadata(gomock.Any()).Return("mockMetadata", nil),
				)
			},
			wantedMetadata: "mockMetadata",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockcfn := mocks.NewMockcfn(ctrl)
			mocks := stackDescriberMocks{
				cfn: mockcfn,
			}

			tc.setupMocks(mocks)

			d := &StackDescriber{
				name: mockStackName,
				cfn:  mockcfn,
			}

			// WHEN
			actual, err := d.StackMetadata()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedMetadata, actual)
			}
		})
	}
}
