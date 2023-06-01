// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type staticSiteDescriberMocks struct {
	wkldDescriber *mocks.MockworkloadDescriber
	store         *mocks.MockDeployedEnvServicesLister
}

func TestStaticSiteDescriber_URI(t *testing.T) {
	const (
		mockApp = "phonetool"
		mockEnv = "test"
		mockSvc = "static"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
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
				URI:        "https://dut843shvcmvn.cloudfront.net/",
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
				URI:        "https://dut843shvcmvn.cloudfront.net/ or https://example.com/",
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

func TestStaticSiteDescriber_Describe(t *testing.T) {
	const (
		mockApp = "phonetool"
		mockEnv = "test"
		mockSvc = "static"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks staticSiteDescriberMocks)

		wantedHuman string
		wantedJSON  string
		wantedError error
	}{
		"return error if fail to list environments": {
			setupMocks: func(m staticSiteDescriberMocks) {
				gomock.InOrder(
					m.store.EXPECT().ListEnvironmentsDeployedTo(mockApp, mockSvc).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf(`list deployed environments for service "static": some error`),
		},
		"success without resources flag": {
			setupMocks: func(m staticSiteDescriberMocks) {
				gomock.InOrder(
					m.store.EXPECT().ListEnvironmentsDeployedTo(mockApp, mockSvc).Return([]string{"test"}, nil),
					m.wkldDescriber.EXPECT().Outputs().Return(map[string]string{
						"CloudFrontDistributionDomainName": "dut843shvcmvn.cloudfront.net",
					}, nil),
				)
			},
			wantedHuman: `About

  Application  phonetool
  Name         static
  Type         Static Site

Routes

  Environment  URL
  -----------  ---
  test         https://dut843shvcmvn.cloudfront.net/
`,
			wantedJSON: "{\"service\":\"static\",\"type\":\"Static Site\",\"application\":\"phonetool\",\"routes\":[{\"environment\":\"test\",\"url\":\"https://dut843shvcmvn.cloudfront.net/\"}]}\n",
		},
		"return an error if failed to get stack resources": {
			shouldOutputResources: true,
			setupMocks: func(m staticSiteDescriberMocks) {
				gomock.InOrder(
					m.store.EXPECT().ListEnvironmentsDeployedTo(mockApp, mockSvc).Return([]string{"test"}, nil),
					m.wkldDescriber.EXPECT().Outputs().Return(map[string]string{
						"CloudFrontDistributionDomainName": "dut843shvcmvn.cloudfront.net",
					}, nil),
					m.wkldDescriber.EXPECT().StackResources().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service resources: some error"),
		},
		"success with resources flag": {
			shouldOutputResources: true,
			setupMocks: func(m staticSiteDescriberMocks) {
				gomock.InOrder(
					m.store.EXPECT().ListEnvironmentsDeployedTo(mockApp, mockSvc).Return([]string{"test"}, nil),
					m.wkldDescriber.EXPECT().Outputs().Return(map[string]string{
						"CloudFrontDistributionDomainName": "dut843shvcmvn.cloudfront.net",
					}, nil),
					m.wkldDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::S3::Bucket",
							PhysicalID: "demo-test-mystatic-bucket-h69vu7y72ga9",
							LogicalID:  "Bucket",
						},
						{
							Type:       "AWS::S3::BucketPolicy",
							PhysicalID: "demo-test-mystatic-BucketPolicyForCloudFront-8AITX9Q7K13R",
							LogicalID:  "BucketPolicy",
						},
					}, nil),
				)
			},
			wantedHuman: `About

  Application  phonetool
  Name         static
  Type         Static Site

Routes

  Environment  URL
  -----------  ---
  test         https://dut843shvcmvn.cloudfront.net/

Resources

  test
    AWS::S3::Bucket        demo-test-mystatic-bucket-h69vu7y72ga9
    AWS::S3::BucketPolicy  demo-test-mystatic-BucketPolicyForCloudFront-8AITX9Q7K13R
`,
			wantedJSON: "{\"service\":\"static\",\"type\":\"Static Site\",\"application\":\"phonetool\",\"routes\":[{\"environment\":\"test\",\"url\":\"https://dut843shvcmvn.cloudfront.net/\"}],\"resources\":{\"test\":[{\"type\":\"AWS::S3::Bucket\",\"physicalID\":\"demo-test-mystatic-bucket-h69vu7y72ga9\",\"logicalID\":\"Bucket\"},{\"type\":\"AWS::S3::BucketPolicy\",\"physicalID\":\"demo-test-mystatic-BucketPolicyForCloudFront-8AITX9Q7K13R\",\"logicalID\":\"BucketPolicy\"}]}}\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := staticSiteDescriberMocks{
				store:         mocks.NewMockDeployedEnvServicesLister(ctrl),
				wkldDescriber: mocks.NewMockworkloadDescriber(ctrl),
			}

			tc.setupMocks(mocks)

			d := &StaticSiteDescriber{
				app:                    mockApp,
				svc:                    mockSvc,
				enableResources:        tc.shouldOutputResources,
				store:                  mocks.store,
				initWkldStackDescriber: func(string) (workloadDescriber, error) { return mocks.wkldDescriber, nil },
				wkldDescribers:         make(map[string]workloadDescriber),
			}

			// WHEN
			got, err := d.Describe()
			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedHuman, got.HumanString())
				gotJSON, _ := got.JSONString()
				require.Equal(t, tc.wantedJSON, gotJSON)
			}
		})
	}
}
