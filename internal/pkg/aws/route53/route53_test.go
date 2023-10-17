// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package route53

import (
	"errors"
	"fmt"
	"net"
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
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
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
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
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
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
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
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
						},
						{
							Name: aws.String("mockDomain3.com"),
							Id:   aws.String("mockID3"),
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
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
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
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
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
						},
						{
							Name: aws.String("mockDomain3.com"),
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
						},
					},
				}, nil)
			},
			wantErr: &ErrDomainHostedZoneNotFound{
				domainName: "mockDomain4.com",
			},
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
		"filter and pick the first public hosted zone": {
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
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
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
							Name: aws.String("mockDomain3.com"),
							Id:   aws.String("mockID2"),
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(true),
							},
						},
						{
							Name: aws.String("mockDomain3.com"),
							Id:   aws.String("mockID3"),
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
						},
						{
							Name: aws.String("mockDomain3.com"),
							Id:   aws.String("mockID4"),
							Config: &route53.HostedZoneConfig{
								PrivateZone: aws.Bool(false),
							},
						},
					},
				}, nil)
			},
			wantHostedZoneID: "mockID3",
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
				client:          mockRoute53Client,
				hostedZoneIDFor: make(map[string]string),
			}

			gotID, gotErr := service.PublicDomainHostedZoneID(tc.domainName)

			if gotErr != nil {
				require.EqualError(t, tc.wantErr, gotErr.Error())
			} else {
				require.Equal(t, tc.wantHostedZoneID, gotID)
			}
		})
	}

	t.Run("should only call route53 once for the same hosted zone", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		m := mocks.NewMockapi(ctrl)
		m.EXPECT().ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
			DNSName: aws.String("example.com"),
		}).Return(&route53.ListHostedZonesByNameOutput{
			IsTruncated: aws.Bool(false),
			HostedZones: []*route53.HostedZone{
				{
					Name: aws.String("example.com"),
					Id:   aws.String("Z0698117FUWMJ87C39TF"),
					Config: &route53.HostedZoneConfig{
						PrivateZone: aws.Bool(false),
					},
				},
			},
		}, nil).Times(1)

		service := Route53{
			client:          m,
			hostedZoneIDFor: make(map[string]string),
		}

		// Call once and make the request.
		actual, err := service.PublicDomainHostedZoneID("example.com")
		require.NoError(t, err)
		require.Equal(t, "Z0698117FUWMJ87C39TF", actual)

		// Call again and Times should be 1.
		actual, err = service.PublicDomainHostedZoneID("example.com")
		require.NoError(t, err)
		require.Equal(t, "Z0698117FUWMJ87C39TF", actual)
	})
}

func TestRoute53_ValidateDomainOwnership(t *testing.T) {
	t.Run("should return a wrapped error if resource record sets cannot be retrieved", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAWS := mocks.NewMockapi(ctrl)
		mockAWS.EXPECT().ListResourceRecordSets(gomock.Any()).Return(nil, errors.New("some error"))
		r53 := Route53{
			client: mockAWS,
			hostedZoneIDFor: map[string]string{
				"example.com": "Z0698117FUWMJ87C39TF",
			},
		}

		// WHEN
		err := r53.ValidateDomainOwnership("example.com")

		// THEN
		require.EqualError(t, err, `list resource record sets for hosted zone ID "Z0698117FUWMJ87C39TF": some error`)
	})
	t.Run("should return a wrapped error if the DNS look up for name server records fails", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAWS := mocks.NewMockapi(ctrl)
		mockAWS.EXPECT().ListResourceRecordSets(gomock.Any()).Return(&route53.ListResourceRecordSetsOutput{}, nil)

		mockResolver := mocks.NewMocknameserverResolver(ctrl)
		mockResolver.EXPECT().LookupNS(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
		r53 := Route53{
			client: mockAWS,
			dns:    mockResolver,
			hostedZoneIDFor: map[string]string{
				"example.com": "Z0698117FUWMJ87C39TF",
			},
		}

		// WHEN
		err := r53.ValidateDomainOwnership("example.com")

		// THEN
		require.EqualError(t, err, `look up NS records for domain "example.com": some error`)
	})
	t.Run("should return ErrUnmatchedNSRecords if the records are not equal", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAWS := mocks.NewMockapi(ctrl)
		mockAWS.EXPECT().ListResourceRecordSets(gomock.Any()).Return(&route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []*route53.ResourceRecordSet{
				{
					Name: aws.String("example.com."),
					Type: aws.String("NS"),
					ResourceRecords: []*route53.ResourceRecord{
						{
							Value: aws.String("ns-1119.awsdns-11.org."),
						},
						{
							Value: aws.String("ns-501.awsdns-62.com."),
						},
					},
				},
			},
		}, nil)

		mockResolver := mocks.NewMocknameserverResolver(ctrl)
		mockResolver.EXPECT().LookupNS(gomock.Any(), gomock.Any()).Return([]*net.NS{
			{
				Host: "dns-ns2.amazon.com.",
			},
			{
				Host: "dns-ns1.amazon.com.",
			},
		}, nil)
		r53 := Route53{
			client: mockAWS,
			dns:    mockResolver,
			hostedZoneIDFor: map[string]string{
				"example.com": "Z0698117FUWMJ87C39TF",
			},
		}

		// WHEN
		err := r53.ValidateDomainOwnership("example.com")

		// THEN
		var wanted *ErrUnmatchedNSRecords
		require.ErrorAs(t, err, &wanted)
	})
	t.Run("should return nil when NS records match exactly", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAWS := mocks.NewMockapi(ctrl)
		mockAWS.EXPECT().ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
			HostedZoneId: aws.String("Z0698117FUWMJ87C39TF"),
		}).Return(&route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []*route53.ResourceRecordSet{
				{
					Name: aws.String("example.com."),
					Type: aws.String("NS"),
					ResourceRecords: []*route53.ResourceRecord{
						{
							Value: aws.String("dns-ns1.amazon.com."),
						},
						{
							Value: aws.String("dns-ns2.amazon.com"),
						},
					},
				},
				{
					Name: aws.String("demo.example.com."),
					Type: aws.String("NS"),
					ResourceRecords: []*route53.ResourceRecord{
						{
							Value: aws.String("ns-473.awsdns-59.com"),
						},
					},
				},
			},
		}, nil)

		mockResolver := mocks.NewMocknameserverResolver(ctrl)
		mockResolver.EXPECT().LookupNS(gomock.Any(), "example.com").Return([]*net.NS{
			{
				Host: "dns-ns2.amazon.com.",
			},
			{
				Host: "dns-ns1.amazon.com",
			},
		}, nil)
		r53 := Route53{
			client: mockAWS,
			dns:    mockResolver,
			hostedZoneIDFor: map[string]string{
				"example.com": "Z0698117FUWMJ87C39TF",
			},
		}

		// WHEN
		err := r53.ValidateDomainOwnership("example.com")

		// THEN
		require.NoError(t, err)
	})
	t.Run("should return nil when ns records are just a subset", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAWS := mocks.NewMockapi(ctrl)
		mockAWS.EXPECT().ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
			HostedZoneId: aws.String("Z0698117FUWMJ87C39TF"),
		}).Return(&route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []*route53.ResourceRecordSet{
				{
					Name: aws.String("example.com."),
					Type: aws.String("NS"),
					ResourceRecords: []*route53.ResourceRecord{
						{
							Value: aws.String("dns-ns1.amazon.com."),
						},
						{
							Value: aws.String("dns-ns2.amazon.com."),
						},
						{
							Value: aws.String("dns-ns3.amazon.com."),
						},
						{
							Value: aws.String("dns-ns4.amazon.com."),
						},
					},
				},
			},
		}, nil)

		mockResolver := mocks.NewMocknameserverResolver(ctrl)
		mockResolver.EXPECT().LookupNS(gomock.Any(), "example.com").Return([]*net.NS{
			{
				Host: "dns-ns2.amazon.com.",
			},
			{
				Host: "dns-ns1.amazon.com",
			},
		}, nil)
		r53 := Route53{
			client: mockAWS,
			dns:    mockResolver,
			hostedZoneIDFor: map[string]string{
				"example.com": "Z0698117FUWMJ87C39TF",
			},
		}

		// WHEN
		err := r53.ValidateDomainOwnership("example.com")

		// THEN
		require.NoError(t, err)
	})
}
