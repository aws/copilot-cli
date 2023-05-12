// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type staticSiteDescriberMocks struct {
	wkldDescriber *mocks.MockworkloadDescriber
}

func TestStaticSiteDescriber_URI(t *testing.T) {
	const (
		mockApp = "phonetool"
		mockEnv = "test"
		mockSvc = "static"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks staticSiteDescriberMocks)

		wantedURI   URI
		wantedError error
	}{
		"return error if fail to get stack output": {
			setupMocks: func(m staticSiteDescriberMocks) {
				gomock.InOrder(
					m.wkldDescriber.EXPECT().Outputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf(`get stack output for service "static": some error`),
		},
		"success without alt domain name": {
			setupMocks: func(m staticSiteDescriberMocks) {
				gomock.InOrder(
					m.wkldDescriber.EXPECT().Outputs().Return(map[string]string{
						"CloudFrontDistributionDomainName": "dut843shvcmvn.cloudfront.net",
					}, nil),
				)
			},
			wantedURI: URI{
				URI:        "dut843shvcmvn.cloudfront.net",
				AccessType: URIAccessTypeInternet,
			},
		},
		"success": {
			setupMocks: func(m staticSiteDescriberMocks) {
				gomock.InOrder(
					m.wkldDescriber.EXPECT().Outputs().Return(map[string]string{
						"CloudFrontDistributionDomainName":            "dut843shvcmvn.cloudfront.net",
						"CloudFrontDistributionAlternativeDomainName": "example.com",
					}, nil),
				)
			},
			wantedURI: URI{
				URI:        "dut843shvcmvn.cloudfront.net or example.com",
				AccessType: URIAccessTypeInternet,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := staticSiteDescriberMocks{
				wkldDescriber: mocks.NewMockworkloadDescriber(ctrl),
			}

			tc.setupMocks(mocks)

			d := &StaticSiteDescriber{
				app:                    mockApp,
				svc:                    mockSvc,
				initWkldStackDescriber: func(string) (workloadDescriber, error) { return mocks.wkldDescriber, nil },
				wkldDescribers:         make(map[string]workloadDescriber),
			}

			// WHEN
			gotURI, err := d.URI(mockEnv)
			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedURI, gotURI)
			}
		})
	}
}
