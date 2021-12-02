// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package route53

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53domains"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/aws/route53/mocks"
)

func TestRoute53_IsRegisteredDomain(t *testing.T) {

	testCases := map[string]struct {
		domainName        string
		mockRoute53Client func(m *mocks.MockdomainAPI)

		wantErr error
	}{
		"domain registered by account in Route53": {
			domainName: "mockDomain.com",
			mockRoute53Client: func(m *mocks.MockdomainAPI) {
				m.EXPECT().GetDomainDetail(&route53domains.GetDomainDetailInput{
					DomainName: aws.String("mockDomain.com"),
				}).Return(nil, nil)
			},
			wantErr: nil,
		},
		"domain not found in Route53": {
			domainName: "mockDomain.com",
			mockRoute53Client: func(m *mocks.MockdomainAPI) {
				m.EXPECT().GetDomainDetail(&route53domains.GetDomainDetailInput{
					DomainName: aws.String("mockDomain.com"),
				}).Return(nil, &route53domains.InvalidInput{
					Message_: aws.String("InvalidInput: Domain mockDomain.com not found in account xxx"),
				})
			},
			wantErr: errors.New("domain mockDomain.com is not found in the account"),
		},
		"domain cannot have been registered in Route53 because the TLD is not supported": {
			domainName: "mockDomain.com",
			mockRoute53Client: func(m *mocks.MockdomainAPI) {
				m.EXPECT().GetDomainDetail(&route53domains.GetDomainDetailInput{
					DomainName: aws.String("mockDomain.com"),
				}).Return(nil, &route53domains.UnsupportedTLD{})
			},
			wantErr: errors.New("domain mockDomain.com is not found in the account"),
		},
		"fail with other invalid input error": {
			domainName: "mockDomain.com",
			mockRoute53Client: func(m *mocks.MockdomainAPI) {
				m.EXPECT().GetDomainDetail(&route53domains.GetDomainDetailInput{
					DomainName: aws.String("mockDomain.com"),
				}).Return(nil, errors.New("InvalidInput: Invalid domain.Errors: [Domain label is empty for domain : [.com] ]"))
			},
			wantErr: errors.New("get domain detail: InvalidInput: Invalid domain.Errors: [Domain label is empty for domain : [.com] ]"),
		},
		"error getting domain detail": {
			domainName: "mockDomain.com",
			mockRoute53Client: func(m *mocks.MockdomainAPI) {
				m.EXPECT().GetDomainDetail(&route53domains.GetDomainDetailInput{
					DomainName: aws.String("mockDomain.com"),
				}).Return(nil, errors.New("some error"))
			},
			wantErr: errors.New("get domain detail: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRoute53Client := mocks.NewMockdomainAPI(ctrl)
			tc.mockRoute53Client(mockRoute53Client)

			client := Route53Domains{
				client: mockRoute53Client,
			}
			gotErr := client.IsRegisteredDomain(tc.domainName)

			if tc.wantErr != nil {
				require.NotNil(t, gotErr)
				require.EqualError(t, tc.wantErr, gotErr.Error())
			}
		})

	}
}
