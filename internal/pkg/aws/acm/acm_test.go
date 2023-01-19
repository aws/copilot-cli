// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package acm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/copilot-cli/internal/pkg/aws/acm/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type acmMocks struct {
	client *mocks.Mockapi
}

func TestACM_ValidateCertAliases(t *testing.T) {
	const ()
	mockError := errors.New("some error")

	testCases := map[string]struct {
		setupMocks func(m acmMocks)
		inAliases  []string
		inCerts    []string

		wantErr        error
		wantAlarmNames []string
	}{
		"errors if failed to describe certificates": {
			inAliases: []string{"copilot.com"},
			inCerts:   []string{"mockCertARN"},
			setupMocks: func(m acmMocks) {
				m.client.EXPECT().DescribeCertificateWithContext(gomock.Any(), &acm.DescribeCertificateInput{
					CertificateArn: aws.String("mockCertARN"),
				}).Return(nil, mockError)
			},

			wantErr: fmt.Errorf("describe certificate mockCertARN: some error"),
		},
		"invalid alias": {
			inAliases: []string{"v1.copilot.com", "myapp.v1.copilot.com"},
			inCerts:   []string{"mockCertARN"},
			setupMocks: func(m acmMocks) {
				m.client.EXPECT().DescribeCertificateWithContext(gomock.Any(), &acm.DescribeCertificateInput{
					CertificateArn: aws.String("mockCertARN"),
				}).Return(&acm.DescribeCertificateOutput{
					Certificate: &acm.CertificateDetail{
						SubjectAlternativeNames: aws.StringSlice([]string{"example.com", "*.copilot.com"}),
					},
				}, nil)
			},

			wantErr: fmt.Errorf("myapp.v1.copilot.com is not a valid domain against mockCertARN"),
		},
		"success": {
			inAliases: []string{"v1.copilot.com", "example.com"},
			inCerts:   []string{"mockCertARN1", "mockCertARN2"},
			setupMocks: func(m acmMocks) {
				m.client.EXPECT().DescribeCertificateWithContext(gomock.Any(), &acm.DescribeCertificateInput{
					CertificateArn: aws.String("mockCertARN1"),
				}).Return(&acm.DescribeCertificateOutput{
					Certificate: &acm.CertificateDetail{
						SubjectAlternativeNames: aws.StringSlice([]string{"copilot.com", "*.copilot.com"}),
					},
				}, nil)
				m.client.EXPECT().DescribeCertificateWithContext(gomock.Any(), &acm.DescribeCertificateInput{
					CertificateArn: aws.String("mockCertARN2"),
				}).Return(&acm.DescribeCertificateOutput{
					Certificate: &acm.CertificateDetail{
						SubjectAlternativeNames: aws.StringSlice([]string{"example.com"}),
					},
				}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockapi(ctrl)
			mocks := acmMocks{
				client: mockClient,
			}

			tc.setupMocks(mocks)

			acmSvc := ACM{
				client: mockClient,
			}

			gotErr := acmSvc.ValidateCertAliases(tc.inAliases, tc.inCerts)

			if gotErr != nil {
				require.EqualError(t, tc.wantErr, gotErr.Error())
			} else {
				require.NoError(t, tc.wantErr)
			}
		})

	}
}
