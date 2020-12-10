// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package iam

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/copilot-cli/internal/pkg/aws/iam/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestIAM_DeleteRole(t *testing.T) {
	testCases := map[string]struct {
		inRoleARN string
		inClient  func(ctrl *gomock.Controller) *mocks.Mockapi

		wantedErr error
	}{
		"wraps error when cannot list role policies": {
			inRoleARN: "arn:aws:iam::1111:role/phonetool-test-CFNExecutionRole",
			inClient: func(ctrl *gomock.Controller) *mocks.Mockapi {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().
					ListRolePolicies(gomock.Any()).
					Return(nil, errors.New("some error"))
				m.EXPECT().DeleteRolePolicy(gomock.Any()).Times(0)
				return m
			},
			wantedErr: errors.New("list role policies for role phonetool-test-CFNExecutionRole: some error"),
		},
		"wraps error when cannot delete role policies": {
			inRoleARN: "arn:aws:iam::1111:role/phonetool-test-CFNExecutionRole",
			inClient: func(ctrl *gomock.Controller) *mocks.Mockapi {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().
					ListRolePolicies(gomock.Any()).
					Return(&iam.ListRolePoliciesOutput{
						PolicyNames: []*string{aws.String("policy1")},
					}, nil)
				m.EXPECT().DeleteRolePolicy(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: errors.New("delete policy named policy1 in role phonetool-test-CFNExecutionRole: some error"),
		},
		"wraps error when cannot delete role": {
			inRoleARN: "arn:aws:iam::1111:role/phonetool-test-CFNExecutionRole",
			inClient: func(ctrl *gomock.Controller) *mocks.Mockapi {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().
					ListRolePolicies(gomock.Any()).
					Return(&iam.ListRolePoliciesOutput{}, nil)
				m.EXPECT().DeleteRole(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: errors.New("delete role named phonetool-test-CFNExecutionRole: some error"),
		},
		"returns nil if the role does not exist": {
			inRoleARN: "arn:aws:iam::1111:role/phonetool-test-CFNExecutionRole",
			inClient: func(ctrl *gomock.Controller) *mocks.Mockapi {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().
					ListRolePolicies(gomock.Any()).
					Return(nil, awserr.New(iam.ErrCodeNoSuchEntityException, "does not exist", nil))
				m.EXPECT().DeleteRole(gomock.Any()).Return(nil, awserr.New(iam.ErrCodeNoSuchEntityException, "does not exist", nil))
				return m
			},
		},
		"returns nil when the role policies and the role can be deleted successfully": {
			inRoleARN: "arn:aws:iam::1111:role/phonetool-test-CFNExecutionRole",
			inClient: func(ctrl *gomock.Controller) *mocks.Mockapi {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().
					ListRolePolicies(&iam.ListRolePoliciesInput{
						RoleName: aws.String("phonetool-test-CFNExecutionRole"),
					}).
					Return(&iam.ListRolePoliciesOutput{
						PolicyNames: []*string{aws.String("policy1"), aws.String("policy2")},
					}, nil)
				gomock.InOrder(
					m.EXPECT().DeleteRolePolicy(&iam.DeleteRolePolicyInput{
						PolicyName: aws.String("policy1"),
						RoleName:   aws.String("phonetool-test-CFNExecutionRole"),
					}).Return(nil, nil),
					m.EXPECT().DeleteRolePolicy(&iam.DeleteRolePolicyInput{
						PolicyName: aws.String("policy2"),
						RoleName:   aws.String("phonetool-test-CFNExecutionRole"),
					}).Return(nil, nil),
				)
				m.EXPECT().DeleteRole(&iam.DeleteRoleInput{
					RoleName: aws.String("phonetool-test-CFNExecutionRole"),
				}).Return(nil, nil)
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			iam := &IAM{
				client: tc.inClient(ctrl),
			}

			// WHEN
			err := iam.DeleteRole(tc.inRoleARN)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIAM_CreateECSServiceLinkedRole(t *testing.T) {
	testCases := map[string]struct {
		inClient func(ctrl *gomock.Controller) *mocks.Mockapi

		wantedErr error
	}{
		"wraps error on failure": {
			inClient: func(ctrl *gomock.Controller) *mocks.Mockapi {
				m := mocks.NewMockapi(ctrl)
				m.EXPECT().
					CreateServiceLinkedRole(gomock.Any()).
					Return(nil, errors.New("some error"))
				return m
			},

			wantedErr: errors.New("create service linked role for Amazon ECS: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			iam := &IAM{
				client: tc.inClient(ctrl),
			}

			// WHEN
			err := iam.CreateECSServiceLinkedRole()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
