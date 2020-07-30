// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ec2

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2/mocks"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	inAppEnvFilters = []Filter{
		{
			Name:   fmt.Sprintf(TagFilterName, deploy.AppTagKey),
			Values: []string{"my-app"},
		},
		{
			Name:   fmt.Sprintf(TagFilterName, deploy.EnvTagKey),
			Values: []string{"my-env"},
		},
	}

	subnet1 = &ec2.Subnet{
		SubnetId:            aws.String("subnet-1"),
		MapPublicIpOnLaunch: aws.Bool(false),
	}
	subnet2 = &ec2.Subnet{
		SubnetId:            aws.String("subnet-2"),
		MapPublicIpOnLaunch: aws.Bool(true),
	}
	subnet3 = &ec2.Subnet{
		SubnetId:            aws.String("subnet-3"),
		MapPublicIpOnLaunch: aws.Bool(true),
	}
)

func TestEC2_ListVpcSubnets(t *testing.T) {
	const mockVpcID = "mockVpcID"
	testCases := map[string]struct {
		mockEC2Client func(m *mocks.Mockapi)

		wantedError   error
		wantedSubnets *Subnets
	}{
		"fail to describe subnets": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSubnets(gomock.Any()).Return(nil, errors.New("error describing subnets"))
			},
			wantedError: fmt.Errorf("describe subnets: error describing subnets"),
		},
		"success": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: toEC2Filter([]Filter{
						{
							Name:   "vpc-id",
							Values: []string{mockVpcID},
						},
					}),
				}).Return(&ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{
						subnet1,
						subnet2,
						subnet3,
					}}, nil)
			},
			wantedSubnets: &Subnets{
				PublicSubnets:  []string{"subnet-2", "subnet-3"},
				PrivateSubnets: []string{"subnet-1"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockAPI := mocks.NewMockapi(ctrl)
			tc.mockEC2Client(mockAPI)

			ec2Client := EC2{
				client: mockAPI,
			}

			subnets, err := ec2Client.ListVpcSubnets(mockVpcID)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedSubnets, subnets)
			}
		})
	}
}
func TestEC2_PublicSubnetIDs(t *testing.T) {
	testCases := map[string]struct {
		inFilter []Filter

		mockEC2Client func(m *mocks.Mockapi)

		wantedError error
		wantedARNs  []string
	}{
		"fail to get public subnets": {
			inFilter: inAppEnvFilters,
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: toEC2Filter(inAppEnvFilters),
				}).Return(nil, errors.New("error describing subnets"))
			},
			wantedError: fmt.Errorf("describe subnets: error describing subnets"),
		},
		"successfully get only public subnets": {
			inFilter: inAppEnvFilters,
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: toEC2Filter(inAppEnvFilters),
				}).Return(&ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{
						subnet1,
						subnet2,
						subnet3,
					}}, nil)
			},
			wantedARNs: []string{"subnet-2", "subnet-3"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockAPI := mocks.NewMockapi(ctrl)
			tc.mockEC2Client(mockAPI)

			ec2Client := EC2{
				client: mockAPI,
			}

			arns, err := ec2Client.PublicSubnetIDs(tc.inFilter...)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedARNs, arns)
			}
		})
	}
}

func TestEC2_SubnetIDs(t *testing.T) {
	mockNextToken := aws.String("mockNextToken")
	testCases := map[string]struct {
		inFilter []Filter

		mockEC2Client func(m *mocks.Mockapi)

		wantedError error
		wantedARNs  []string
	}{
		"failed to get subnets": {
			inFilter: inAppEnvFilters,
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: toEC2Filter(inAppEnvFilters),
				}).Return(nil, errors.New("error describing subnets"))
			},
			wantedError: fmt.Errorf("describe subnets: error describing subnets"),
		},
		"successfully get subnets": {
			inFilter: inAppEnvFilters,
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: toEC2Filter(inAppEnvFilters),
				}).Return(&ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{
						subnet1, subnet2,
					},
				}, nil)
			},
			wantedARNs: []string{"subnet-1", "subnet-2"},
		},
		"successfully get subnets with pagination": {
			inFilter: inAppEnvFilters,
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: toEC2Filter(inAppEnvFilters),
				}).Return(&ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{
						subnet1, subnet2,
					},
					NextToken: mockNextToken,
				}, nil)
				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: toEC2Filter(inAppEnvFilters),
				}).Return(&ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{
						subnet3,
					},
				}, nil)
			},
			wantedARNs: []string{"subnet-1", "subnet-2", "subnet-3"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockAPI := mocks.NewMockapi(ctrl)
			tc.mockEC2Client(mockAPI)

			ec2Client := EC2{
				client: mockAPI,
			}

			arns, err := ec2Client.SubnetIDs(tc.inFilter...)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedARNs, arns)
			}
		})
	}
}

func TestEC2_SecurityGroups(t *testing.T) {
	testCases := map[string]struct {
		inFilter []Filter

		mockEC2Client func(m *mocks.Mockapi)

		wantedError error
		wantedARNs  []string
	}{
		"failed to get security groups": {
			inFilter: inAppEnvFilters,
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: toEC2Filter(inAppEnvFilters),
				}).Return(nil, errors.New("error getting security groups"))
			},

			wantedError: errors.New("describe security groups: error getting security groups"),
		},
		"get security groups success": {
			inFilter: inAppEnvFilters,
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: toEC2Filter(inAppEnvFilters),
				}).Return(&ec2.DescribeSecurityGroupsOutput{
					SecurityGroups: []*ec2.SecurityGroup{
						{
							GroupId: aws.String("sg-1"),
						},
						{
							GroupId: aws.String("sg-2"),
						},
					},
				}, nil)
			},

			wantedARNs: []string{"sg-1", "sg-2"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockAPI := mocks.NewMockapi(ctrl)
			tc.mockEC2Client(mockAPI)

			ec2Client := EC2{
				client: mockAPI,
			}

			arns, err := ec2Client.SecurityGroups(inAppEnvFilters...)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedARNs, arns)
			}
		})
	}
}
