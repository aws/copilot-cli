// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package route53

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/copilot-cli/internal/pkg/aws/route53/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestRoute53_DomainHostedZoneID(t *testing.T) {

	testCases := map[string]struct {
		domainName        string
		mockRoute53Client func(m *mocks.Mockapi)

		wantErr          error
		wantHostedZoneID string
	}{
		"domain exists": {
			domainName: "mockDomain.com",
			mockRoute53Client: func(m *mocks.Mockapi) {
				m.EXPECT().ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
					DNSName: aws.String("mockDomain.com"),
				}).Return(&route53.ListHostedZonesByNameOutput{
					IsTruncated: aws.Bool(false),
					HostedZones: []*route53.HostedZone{
						{
							Name: aws.String("mockDomain.com"),
							Id:   aws.String("mockID"),
						},
					},
				}, nil)
			},
			wantHostedZoneID: "mockID",
			wantErr:          nil,
		},
		"DNS with subdomain": {
			domainName: "mockDomain.subdomain.com",
			mockRoute53Client: func(m *mocks.Mockapi) {
				m.EXPECT().ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
					DNSName: aws.String("mockDomain.subdomain.com"),
				}).Return(&route53.ListHostedZonesByNameOutput{
					IsTruncated: aws.Bool(false),
					HostedZones: []*route53.HostedZone{
						{
							Name: aws.String("mockDomain.subdomain.com."),
							Id:   aws.String("mockID"),
						},
					},
				}, nil)
			},
			wantHostedZoneID: "mockID",
			wantErr:          nil,
		},
		"domain exists within more than one page": {
			domainName: "mockDomain3.com",
			mockRoute53Client: func(m *mocks.Mockapi) {
				m.EXPECT().ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
					DNSName: aws.String("mockDomain3.com"),
				}).Return(&route53.ListHostedZonesByNameOutput{
					IsTruncated:      aws.Bool(true),
					NextDNSName:      aws.String("mockDomain2.com"),
					NextHostedZoneId: aws.String("mockID"),
					HostedZones: []*route53.HostedZone{
						{
							Name: aws.String("mockDomain1.com"),
							Id:   aws.String("mockID1"),
						},
					},
				}, nil)
				m.EXPECT().ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
					DNSName:      aws.String("mockDomain2.com"),
					HostedZoneId: aws.String("mockID"),
				}).Return(&route53.ListHostedZonesByNameOutput{
					IsTruncated: aws.Bool(false),
					HostedZones: []*route53.HostedZone{
						{
							Name: aws.String("mockDomain2.com"),
							Id:   aws.String("mockID2"),
						},
						{
							Name: aws.String("mockDomain3.com"),
							Id:   aws.String("mockID3"),
						},
					},
				}, nil)
			},
			wantHostedZoneID: "mockID3",
			wantErr:          nil,
		},
		"domain does not exist": {
			domainName: "mockDomain4.com",
			mockRoute53Client: func(m *mocks.Mockapi) {
				m.EXPECT().ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
					DNSName: aws.String("mockDomain4.com"),
				}).Return(&route53.ListHostedZonesByNameOutput{
					IsTruncated:      aws.Bool(true),
					NextDNSName:      aws.String("mockDomain2.com"),
					NextHostedZoneId: aws.String("mockID"),
					HostedZones: []*route53.HostedZone{
						{
							Name: aws.String("mockDomain1.com"),
						},
					},
				}, nil)
				m.EXPECT().ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
					DNSName:      aws.String("mockDomain2.com"),
					HostedZoneId: aws.String("mockID"),
				}).Return(&route53.ListHostedZonesByNameOutput{
					IsTruncated: aws.Bool(false),
					HostedZones: []*route53.HostedZone{
						{
							Name: aws.String("mockDomain2.com"),
						},
						{
							Name: aws.String("mockDomain3.com"),
						},
					},
				}, nil)
			},
			wantErr: ErrDomainNotExist,
		},
		"failed to validate if domain exists": {
			domainName: "mockDomain.com",
			mockRoute53Client: func(m *mocks.Mockapi) {
				m.EXPECT().ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
					DNSName: aws.String("mockDomain.com"),
				}).Return(nil, errors.New("some error"))
			},
			wantErr: fmt.Errorf("list hosted zone for mockDomain.com: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRoute53Client := mocks.NewMockapi(ctrl)
			tc.mockRoute53Client(mockRoute53Client)

			service := Route53{
				client: mockRoute53Client,
			}

			gotID, gotErr := service.DomainHostedZoneID(tc.domainName)

			if gotErr != nil {
				require.EqualError(t, tc.wantErr, gotErr.Error())
			} else {
				require.Equal(t, tc.wantHostedZoneID, gotID)
			}
		})

	}
}
