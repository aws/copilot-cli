// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/term/selector/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type ec2SelectMocks struct {
	prompt *mocks.MockPrompter
	ec2Svc *mocks.MockVPCSubnetLister
}

func TestEc2Select_VPC(t *testing.T) {
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks ec2SelectMocks)

		wantErr error
		wantVPC string
	}{
		"return error if fail to list VPCs": {
			setupMocks: func(m ec2SelectMocks) {
				m.ec2Svc.EXPECT().ListVPC().Return(nil, mockErr)

			},
			wantErr: fmt.Errorf("list VPC ID: some error"),
		},
		"return error if no VPC found": {
			setupMocks: func(m ec2SelectMocks) {
				m.ec2Svc.EXPECT().ListVPC().Return([]string{}, nil)

			},
			wantErr: ErrVpcNotFound,
		},
		"return error if fail to select a VPC": {
			setupMocks: func(m ec2SelectMocks) {
				m.ec2Svc.EXPECT().ListVPC().Return([]string{"mockVPC1", "mockVPC2"}, nil)
				m.prompt.EXPECT().SelectOne("Select a VPC", "Help text", []string{"mockVPC1", "mockVPC2"}).
					Return("", mockErr)

			},
			wantErr: fmt.Errorf("select VPC: some error"),
		},
		"success": {
			setupMocks: func(m ec2SelectMocks) {
				m.ec2Svc.EXPECT().ListVPC().Return([]string{"mockVPC1", "mockVPC2"}, nil)
				m.prompt.EXPECT().SelectOne("Select a VPC", "Help text", []string{"mockVPC1", "mockVPC2"}).
					Return("mockVPC1", nil)

			},
			wantVPC: "mockVPC1",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockec2Svc := mocks.NewMockVPCSubnetLister(ctrl)
			mockprompt := mocks.NewMockPrompter(ctrl)
			mocks := ec2SelectMocks{
				ec2Svc: mockec2Svc,
				prompt: mockprompt,
			}
			tc.setupMocks(mocks)

			sel := EC2Select{
				prompt: mockprompt,
				ec2Svc: mockec2Svc,
			}
			vpc, err := sel.VPC("Select a VPC", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.wantVPC, vpc)
			}
		})
	}
}

func TestEc2Select_subnets(t *testing.T) {
	mockErr := errors.New("some error")
	mockVPC := "mockVPC"
	testCases := map[string]struct {
		public     bool
		setupMocks func(mocks ec2SelectMocks)

		wantErr     error
		wantSubnets []string
	}{
		"return error if fail to list subnets": {
			setupMocks: func(m ec2SelectMocks) {
				m.ec2Svc.EXPECT().ListVPCSubnets(mockVPC, gomock.Any()).Return(nil, mockErr)
			},
			wantErr: fmt.Errorf("list subnets for VPC mockVPC: some error"),
		},
		"return error if no subnets found": {
			setupMocks: func(m ec2SelectMocks) {
				m.ec2Svc.EXPECT().ListVPCSubnets(mockVPC, gomock.Any()).Return([]string{}, nil)
			},
			wantErr: ErrSubnetsNotFound,
		},
		"return error if fail to select": {
			setupMocks: func(m ec2SelectMocks) {
				m.ec2Svc.EXPECT().ListVPCSubnets(mockVPC, gomock.Any()).Return([]string{"mockSubnet1", "mockSubnet2"}, nil)
				m.prompt.EXPECT().MultiSelect("Select a subnet", "Help text", []string{"mockSubnet1", "mockSubnet2"}).
					Return(nil, mockErr)
			},
			wantErr: fmt.Errorf("some error"),
		},
		"success for public subnets": {
			public: true,
			setupMocks: func(m ec2SelectMocks) {
				m.ec2Svc.EXPECT().ListVPCSubnets(mockVPC, gomock.Any()).Return([]string{"mockSubnet1", "mockSubnet2"}, nil)
				m.prompt.EXPECT().MultiSelect("Select a subnet", "Help text", []string{"mockSubnet1", "mockSubnet2"}).
					Return([]string{"mockSubnet1", "mockSubnet2"}, nil)
			},
			wantSubnets: []string{"mockSubnet1", "mockSubnet2"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockec2Svc := mocks.NewMockVPCSubnetLister(ctrl)
			mockprompt := mocks.NewMockPrompter(ctrl)
			mocks := ec2SelectMocks{
				ec2Svc: mockec2Svc,
				prompt: mockprompt,
			}
			tc.setupMocks(mocks)

			sel := EC2Select{
				prompt: mockprompt,
				ec2Svc: mockec2Svc,
			}
			subnets, err := sel.subnet("Select a subnet", "Help text", mockVPC, tc.public)
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.wantSubnets, subnets)
			}
		})
	}
}
