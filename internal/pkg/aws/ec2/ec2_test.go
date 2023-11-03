// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ec2

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	inAppEnvFilters = []Filter{
		{
			Name:   fmt.Sprintf(FmtTagFilter, "copilot-application"),
			Values: []string{"my-app"},
		},
		{
			Name:   fmt.Sprintf(FmtTagFilter, "copilot-environment"),
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

func TestEC2_FilterForTags(t *testing.T) {
	testCases := map[string]struct {
		inValues []string
		wanted   Filter
	}{
		"with no values": {
			wanted: Filter{
				Name:   "tag-key",
				Values: []string{"mockKey"},
			},
		},
		"with values": {
			inValues: []string{"foo", "bar"},
			wanted: Filter{
				Name:   "tag:mockKey",
				Values: []string{"foo", "bar"},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			filter := FilterForTags("mockKey", tc.inValues...)
			require.Equal(t, tc.wanted, filter)
		})
	}
}

func TestEC2_extractResource(t *testing.T) {
	testCases := map[string]struct {
		displayString  string
		wantedError    error
		wantedResource *Resource
	}{
		"returns error if string is empty": {
			displayString: "",
			wantedError:   fmt.Errorf("extract resource ID from string: "),
		},
		"returns just the VPC ID if no name present": {
			displayString: "vpc-imagr8vpcstring",
			wantedError:   nil,
			wantedResource: &Resource{
				ID: "vpc-imagr8vpcstring",
			},
		},
		"returns both the VPC ID and name if both present": {
			displayString: "vpc-imagr8vpcstring (copilot-app-name-env)",
			wantedError:   nil,
			wantedResource: &Resource{
				ID:   "vpc-imagr8vpcstring",
				Name: "copilot-app-name-env",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			resource, err := extractResource(tc.displayString)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedResource, resource)
			}
		})
	}
}

func TestEC2_ListVPC(t *testing.T) {
	testCases := map[string]struct {
		mockEC2Client func(m *mocks.Mockapi)

		wantedError error
		wantedVPC   []VPC
	}{
		"fail to describe vpcs": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeVpcs(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("describe VPCs: some error"),
		},
		"success": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeVpcs(&ec2.DescribeVpcsInput{}).Return(&ec2.DescribeVpcsOutput{
					Vpcs: []*ec2.Vpc{
						{
							VpcId: aws.String("mockVPCID1"),
						},
					},
					NextToken: aws.String("mockNextToken"),
				}, nil)
				m.EXPECT().DescribeVpcs(&ec2.DescribeVpcsInput{
					NextToken: aws.String("mockNextToken"),
				}).Return(&ec2.DescribeVpcsOutput{
					Vpcs: []*ec2.Vpc{
						{
							VpcId: aws.String("mockVPCID2"),
							Tags: []*ec2.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String("mockVPC2Name"),
								},
							},
						},
					},
				}, nil)
			},
			wantedVPC: []VPC{
				{
					Resource: Resource{
						ID: "mockVPCID1",
					},
				},
				{
					Resource: Resource{
						ID:   "mockVPCID2",
						Name: "mockVPC2Name",
					},
				},
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

			vpcs, err := ec2Client.ListVPCs()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedVPC, vpcs)
			}
		})
	}
}

func TestEC2_ListAZs(t *testing.T) {
	testCases := map[string]struct {
		mockClient func(m *mocks.Mockapi)

		wantedErr string
		wantedAZs []AZ
	}{
		"return wrapped error on unexpected call error": {
			mockClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeAvailabilityZones(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: "describe availability zones: some error",
		},
		"returns AZs that are available and opted-in": {
			mockClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeAvailabilityZones(&ec2.DescribeAvailabilityZonesInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("zone-type"),
							Values: aws.StringSlice([]string{"availability-zone"}),
						},
						{
							Name:   aws.String("state"),
							Values: aws.StringSlice([]string{"available"}),
						},
					},
				}).Return(&ec2.DescribeAvailabilityZonesOutput{
					AvailabilityZones: []*ec2.AvailabilityZone{
						{
							GroupName:          aws.String("us-west-2"),
							NetworkBorderGroup: aws.String("us-west-2"),
							OptInStatus:        aws.String("opt-in-not-required"),
							RegionName:         aws.String("us-west-2"),
							State:              aws.String("available"),
							ZoneId:             aws.String("usw2-az1"),
							ZoneName:           aws.String("us-west-2a"),
							ZoneType:           aws.String("availability-zone"),
						},
						{
							GroupName:          aws.String("us-west-2"),
							NetworkBorderGroup: aws.String("us-west-2"),
							OptInStatus:        aws.String("opt-in-not-required"),
							RegionName:         aws.String("us-west-2"),
							State:              aws.String("available"),
							ZoneId:             aws.String("usw2-az2"),
							ZoneName:           aws.String("us-west-2b"),
							ZoneType:           aws.String("availability-zone"),
						},
					},
				}, nil)
			},
			wantedAZs: []AZ{
				{
					ID:   "usw2-az1",
					Name: "us-west-2a",
				},
				{
					ID:   "usw2-az2",
					Name: "us-west-2b",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := mocks.NewMockapi(ctrl)
			tc.mockClient(m)
			ec2 := EC2{client: m}

			// WHEN
			azs, err := ec2.ListAZs()

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedAZs, azs)
			}
		})
	}
}

func TestEC2_managedPrefixList(t *testing.T) {
	const (
		mockPrefixListName = "mockName"
		mockPrefixListId   = "mockId"
		mockNextToken      = "mockNextToken"
	)
	mockError := errors.New("some error")
	mockFilter := []*ec2.Filter{
		{
			Name:   aws.String("prefix-list-name"),
			Values: aws.StringSlice([]string{mockPrefixListName}),
		},
	}
	mockPrefixList := []*ec2.ManagedPrefixList{
		{
			PrefixListId: aws.String(mockPrefixListId),
		},
	}

	testCases := map[string]struct {
		mockEC2Client func(m *mocks.Mockapi)

		wantedError          error
		wantedErrorMsgPrefix string
		wantedList           *ec2.DescribeManagedPrefixListsOutput
	}{
		"query returns error": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeManagedPrefixLists(gomock.Any()).Return(nil, mockError)
			},
			wantedError: fmt.Errorf("describe managed prefix list with name %s: %w", mockPrefixListName, mockError),
		},
		"query returns Successfully": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeManagedPrefixLists(&ec2.DescribeManagedPrefixListsInput{
					Filters: mockFilter,
				}).Return(&ec2.DescribeManagedPrefixListsOutput{
					NextToken: aws.String(mockNextToken),
					PrefixLists: []*ec2.ManagedPrefixList{
						{
							PrefixListId: aws.String(mockPrefixListId),
						},
					},
				}, nil)
			},
			wantedList: &ec2.DescribeManagedPrefixListsOutput{
				NextToken:   aws.String(mockNextToken),
				PrefixLists: mockPrefixList,
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

			output, err := ec2Client.managedPrefixList(mockPrefixListName)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantedErrorMsgPrefix)
				return
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedList, output, "managed prefix lists output must be equal")
			}
		})
	}
}

func TestEC2_CloudFrontManagedPrefixListId(t *testing.T) {
	const (
		mockPrefixListName = cloudFrontPrefixListName
		mockPrefixListId   = "mockId"
		mockNextToken      = "mockNextToken"
	)
	mockError := errors.New("some error")
	mockFilter := []*ec2.Filter{
		{
			Name:   aws.String("prefix-list-name"),
			Values: aws.StringSlice([]string{mockPrefixListName}),
		},
	}

	testCases := map[string]struct {
		mockEC2Client func(m *mocks.Mockapi)

		wantedError          error
		wantedErrorMsgPrefix string
		wantedId             string
	}{
		"query returns error": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeManagedPrefixLists(gomock.Any()).Return(nil, mockError)
			},
			wantedError: fmt.Errorf("describe managed prefix list with name %s: %w", mockPrefixListName, mockError),
		},
		"query returns no prefix list ids": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeManagedPrefixLists(&ec2.DescribeManagedPrefixListsInput{
					Filters: mockFilter,
				}).Return(&ec2.DescribeManagedPrefixListsOutput{
					NextToken:   aws.String(mockNextToken),
					PrefixLists: []*ec2.ManagedPrefixList{},
				}, nil)
			},
			wantedError: fmt.Errorf("cannot find any prefix list with name: %s", mockPrefixListName),
		},
		"query returns too many prefix list ids": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeManagedPrefixLists(&ec2.DescribeManagedPrefixListsInput{
					Filters: mockFilter,
				}).Return(&ec2.DescribeManagedPrefixListsOutput{
					NextToken: aws.String(mockNextToken),
					PrefixLists: []*ec2.ManagedPrefixList{
						{
							PrefixListId: aws.String(mockPrefixListId),
						},
						{
							PrefixListId: aws.String(mockPrefixListId),
						},
					},
				}, nil)
			},
			wantedErrorMsgPrefix: `found more than one prefix list with the name `,
		},
		"query returns Successfully": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeManagedPrefixLists(&ec2.DescribeManagedPrefixListsInput{
					Filters: mockFilter,
				}).Return(&ec2.DescribeManagedPrefixListsOutput{
					NextToken: aws.String(mockNextToken),
					PrefixLists: []*ec2.ManagedPrefixList{
						{
							PrefixListId: aws.String(mockPrefixListId),
						},
					},
				}, nil)
			},
			wantedId: mockPrefixListId,
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

			id, err := ec2Client.CloudFrontManagedPrefixListID()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantedErrorMsgPrefix)
				return
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedId, id, "ids must be equal")
			}
		})
	}
}

func TestEC2_ListVPCSubnets(t *testing.T) {
	const (
		mockVPCID     = "mockVPC"
		mockNextToken = "mockNextToken"
	)
	mockfilter := []*ec2.Filter{
		{
			Name:   aws.String("vpc-id"),
			Values: aws.StringSlice([]string{mockVPCID}),
		},
	}
	mockError := errors.New("some error")

	testCases := map[string]struct {
		mockEC2Client func(m *mocks.Mockapi)

		wantedError          error
		wantedPublicSubnets  []Subnet
		wantedPrivateSubnets []Subnet
	}{
		"fail to describe route tables": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRouteTables(gomock.Any()).Return(nil, mockError)
			},
			wantedError: fmt.Errorf("describe route tables: some error"),
		},
		"fail to describe subnets": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRouteTables(&ec2.DescribeRouteTablesInput{
					Filters: mockfilter,
				}).Return(&ec2.DescribeRouteTablesOutput{}, nil)
				m.EXPECT().DescribeSubnets(gomock.Any()).Return(nil, mockError)
			},
			wantedError: fmt.Errorf("describe subnets: some error"),
		},
		"can retrieve subnets explicitly associated with an internet gateway": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRouteTables(&ec2.DescribeRouteTablesInput{
					Filters: mockfilter,
				}).Return(&ec2.DescribeRouteTablesOutput{
					RouteTables: []*ec2.RouteTable{
						{
							Associations: []*ec2.RouteTableAssociation{
								{
									SubnetId: aws.String("subnet1"),
								},
							},
							Routes: []*ec2.Route{
								{
									GatewayId: aws.String("local"),
								},
							},
						},
					},
					NextToken: aws.String(mockNextToken),
				}, nil)
				m.EXPECT().DescribeRouteTables(&ec2.DescribeRouteTablesInput{
					Filters:   mockfilter,
					NextToken: aws.String(mockNextToken),
				}).Return(&ec2.DescribeRouteTablesOutput{
					RouteTables: []*ec2.RouteTable{
						{
							Associations: []*ec2.RouteTableAssociation{
								{
									SubnetId: aws.String("subnet2"),
								},
								{
									SubnetId: aws.String("subnet3"),
								},
							},
							Routes: []*ec2.Route{
								{
									GatewayId: aws.String("igw-0333791c413f9e2d8"),
								},
							},
						},
					},
				}, nil)
				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: mockfilter,
				}).Return(&ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{
						{
							SubnetId:  aws.String("subnet1"),
							CidrBlock: aws.String("10.0.0.0/24"),
						},
						{
							SubnetId:  aws.String("subnet2"),
							CidrBlock: aws.String("10.0.1.0/24"),
						},
						{
							SubnetId: aws.String("subnet3"),
							Tags: []*ec2.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String("mySubnet"),
								},
							},
							CidrBlock: aws.String("10.0.2.0/24"),
						},
					},
				}, nil)
			},
			wantedPublicSubnets: []Subnet{
				{
					Resource: Resource{
						ID: "subnet2",
					},
					CIDRBlock: "10.0.1.0/24",
				},
				{
					Resource: Resource{
						ID:   "subnet3",
						Name: "mySubnet",
					},
					CIDRBlock: "10.0.2.0/24",
				},
			},
			wantedPrivateSubnets: []Subnet{
				{
					Resource: Resource{
						ID: "subnet1",
					},
					CIDRBlock: "10.0.0.0/24",
				},
			},
		},
		"can retrieve subnets that are implicitly associated with an internet gateway": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRouteTables(&ec2.DescribeRouteTablesInput{
					Filters: mockfilter,
				}).Return(&ec2.DescribeRouteTablesOutput{
					RouteTables: []*ec2.RouteTable{
						{
							Associations: []*ec2.RouteTableAssociation{
								{
									Main: aws.Bool(true),
								},
							},
							Routes: []*ec2.Route{
								{
									GatewayId:            aws.String("local"),
									DestinationCidrBlock: aws.String("172.31.0.0/16"),
								},
								{
									GatewayId:            aws.String("igw-3542f24c"),
									DestinationCidrBlock: aws.String("0.0.0.0/0"),
								},
							},
						},
					},
				}, nil)

				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: mockfilter,
				}).Return(&ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{
						{
							SubnetId:  aws.String("subnet1"),
							CidrBlock: aws.String("172.31.16.0/20"),
						},
						{
							SubnetId:  aws.String("subnet2"),
							CidrBlock: aws.String("172.31.48.0/20"),
						},
						{
							SubnetId:  aws.String("subnet3"),
							CidrBlock: aws.String("172.31.32.0/20"),
						},
					},
				}, nil)
			},
			wantedPublicSubnets: []Subnet{
				{
					Resource: Resource{
						ID: "subnet1",
					},
					CIDRBlock: "172.31.16.0/20",
				},
				{
					Resource: Resource{
						ID: "subnet2",
					},
					CIDRBlock: "172.31.48.0/20",
				},
				{
					Resource: Resource{
						ID: "subnet3",
					},
					CIDRBlock: "172.31.32.0/20",
				},
			},
			wantedPrivateSubnets: nil,
		},
		"prioritizes explicit route table association over implicit while detecting public subnets": {
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRouteTables(&ec2.DescribeRouteTablesInput{
					Filters: mockfilter,
				}).Return(&ec2.DescribeRouteTablesOutput{
					RouteTables: []*ec2.RouteTable{
						{
							Associations: []*ec2.RouteTableAssociation{
								{
									Main:     aws.Bool(false),
									SubnetId: aws.String("subnet1"),
								},
							},
							Routes: []*ec2.Route{
								{
									GatewayId:            aws.String("local"),
									DestinationCidrBlock: aws.String("172.31.0.0/16"),
								},
							},
						},
						{
							Associations: []*ec2.RouteTableAssociation{
								{
									Main: aws.Bool(true),
								},
							},
							Routes: []*ec2.Route{
								{
									GatewayId:            aws.String("local"),
									DestinationCidrBlock: aws.String("172.31.0.0/16"),
								},
								{
									GatewayId:            aws.String("igw-3542f24c"),
									DestinationCidrBlock: aws.String("0.0.0.0/0"),
								},
							},
						},
					},
				}, nil)

				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: mockfilter,
				}).Return(&ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{
						{
							SubnetId:  aws.String("subnet1"),
							CidrBlock: aws.String("172.31.16.0/20"),
						},
						{
							SubnetId:  aws.String("subnet2"),
							CidrBlock: aws.String("172.31.48.0/20"),
						},
					},
				}, nil)
			},
			wantedPublicSubnets: []Subnet{
				{
					Resource: Resource{
						ID: "subnet2",
					},
					CIDRBlock: "172.31.48.0/20",
				},
			},
			wantedPrivateSubnets: []Subnet{
				{
					Resource: Resource{
						ID: "subnet1",
					},
					CIDRBlock: "172.31.16.0/20",
				},
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

			subnets, err := ec2Client.ListVPCSubnets(mockVPCID)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedPublicSubnets, subnets.Public, "public subnets must equal")
				require.Equal(t, tc.wantedPrivateSubnets, subnets.Private, "private subnets must equal")
			}
		})
	}
}

func TestEC2_PublicIP(t *testing.T) {
	testCases := map[string]struct {
		inENI         string
		mockEC2Client func(m *mocks.Mockapi)

		wantedIP  string
		wantedErr error
	}{
		"failed to describe network interfaces": {
			inENI: "eni-1",
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
					NetworkInterfaceIds: aws.StringSlice([]string{"eni-1"}),
				}).Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("describe network interface with ENI eni-1: some error"),
		},
		"no association information found": {
			inENI: "eni-1",
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
					NetworkInterfaceIds: aws.StringSlice([]string{"eni-1"}),
				}).Return(&ec2.DescribeNetworkInterfacesOutput{
					NetworkInterfaces: []*ec2.NetworkInterface{
						{},
					},
				}, nil)
			},
			wantedErr: errors.New("no association information found for ENI eni-1"),
		},
		"successfully get public ip": {
			inENI: "eni-1",
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
					NetworkInterfaceIds: aws.StringSlice([]string{"eni-1"}),
				}).Return(&ec2.DescribeNetworkInterfacesOutput{
					NetworkInterfaces: []*ec2.NetworkInterface{
						{
							Association: &ec2.NetworkInterfaceAssociation{
								PublicIp: aws.String("1.2.3"),
							},
						},
					},
				}, nil)
			},
			wantedIP: "1.2.3",
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

			out, err := ec2Client.PublicIP(tc.inENI)
			if tc.wantedErr != nil {
				require.EqualError(t, tc.wantedErr, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedIP, out)
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
		"cannot get any subnets": {
			inFilter: inAppEnvFilters,
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: toEC2Filter(inAppEnvFilters),
				}).Return(&ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{},
				}, nil)
			},
			wantedError: fmt.Errorf("cannot find any subnets"),
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
					Filters:   toEC2Filter(inAppEnvFilters),
					NextToken: mockNextToken,
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

func TestEC2_HasDNSSupport(t *testing.T) {
	testCases := map[string]struct {
		vpcID string

		mockEC2Client func(m *mocks.Mockapi)

		wantedError   error
		wantedSupport bool
	}{
		"fail to descibe VPC attribute": {
			vpcID: "mockVPCID",
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeVpcAttribute(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("describe enableDnsSupport attribute for VPC mockVPCID: some error"),
		},
		"success": {
			vpcID: "mockVPCID",
			mockEC2Client: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeVpcAttribute(&ec2.DescribeVpcAttributeInput{
					VpcId:     aws.String("mockVPCID"),
					Attribute: aws.String(ec2.VpcAttributeNameEnableDnsSupport),
				}).Return(&ec2.DescribeVpcAttributeOutput{
					EnableDnsSupport: &ec2.AttributeBooleanValue{
						Value: aws.Bool(true),
					},
				}, nil)
			},
			wantedSupport: true,
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

			support, err := ec2Client.HasDNSSupport(tc.vpcID)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedSupport, support)
			}
		})
	}
}
