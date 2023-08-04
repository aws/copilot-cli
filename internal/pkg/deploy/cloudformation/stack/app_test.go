// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	mockTemplate = "mockTemplate"
)

func TestAppTemplate(t *testing.T) {
	testCases := map[string]struct {
		inVersion        string
		mockDependencies func(ctrl *gomock.Controller, c *AppStackConfig)

		wantedTemplate string
		wantedError    error
	}{
		"should return error given template not found": {
			mockDependencies: func(ctrl *gomock.Controller, c *AppStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				c.parser = m
			},

			wantedError: errors.New("some error"),
		},
		"success": {
			inVersion: "v1.0.0",
			mockDependencies: func(ctrl *gomock.Controller, c *AppStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(appTemplatePath, struct {
					TemplateVersion         string
					AppDNSDelegatedAccounts []string
					Domain                  string
					Name                    string
					PermissionsBoundary     string
				}{
					"v1.0.0",
					[]string{"123456"},
					"",
					"demo",
					"",
				}, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("template"),
				}, nil)
				c.parser = m
			},

			wantedTemplate: "template",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			appStack := &AppStackConfig{
				CreateAppInput: &deploy.CreateAppInput{
					Version:             tc.inVersion,
					AccountID:           "123456",
					Name:                "demo",
					DomainName:          "",
					PermissionsBoundary: "",
				},
			}
			tc.mockDependencies(ctrl, appStack)

			// WHEN
			got, err := appStack.Template()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedTemplate, got)
		})
	}
}

func TestDNSDelegationAccounts(t *testing.T) {
	testCases := map[string]struct {
		given *deploy.CreateAppInput
		want  []string
	}{
		"should append app account": {
			given: &deploy.CreateAppInput{
				AccountID: "1234",
			},
			want: []string{"1234"},
		},
		"should ignore duplicates": {
			given: &deploy.CreateAppInput{
				AccountID:             "1234",
				DNSDelegationAccounts: []string{"1234"},
			},
			want: []string{"1234"},
		},
		"should return a set": {
			given: &deploy.CreateAppInput{
				AccountID:             "1234",
				DNSDelegationAccounts: []string{"4567"},
			},
			want: []string{"1234", "4567"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			appStack := &AppStackConfig{
				CreateAppInput: tc.given,
			}
			got := appStack.dnsDelegationAccounts()
			require.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestAppResourceTemplate(t *testing.T) {
	testCases := map[string]struct {
		given            *AppResourcesConfig
		mockDependencies func(ctrl *gomock.Controller, c *AppStackConfig)

		wantedTemplate string
		wantedError    error
	}{
		"should return error when template cannot be parsed": {
			given: &AppResourcesConfig{},
			mockDependencies: func(ctrl *gomock.Controller, c *AppStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				c.parser = m
			},

			wantedError: errors.New("some error"),
		},
		"should render template after sorting": {
			given: &AppResourcesConfig{
				Accounts: []string{"4567", "1234"},
				Workloads: []AppResourcesWorkload{
					{Name: "svc-2"},
					{Name: "svc-1"},
				},
				Version: 1,
				App:     "testapp",
			},
			mockDependencies: func(ctrl *gomock.Controller, c *AppStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(appResourcesTemplatePath, struct {
					*AppResourcesConfig
					ServiceTagKey   string
					TemplateVersion string
				}{
					&AppResourcesConfig{
						Accounts: []string{"1234", "4567"},
						Workloads: []AppResourcesWorkload{
							{Name: "svc-1"},
							{Name: "svc-2"},
						},
						Version: 1,
						App:     "testapp",
					},
					deploy.ServiceTagKey,
					"",
				}, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("template"),
				}, nil)
				c.parser = m
			},

			wantedTemplate: "template",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			appStack := &AppStackConfig{
				CreateAppInput: &deploy.CreateAppInput{Name: "testapp", AccountID: "1234"},
			}
			tc.mockDependencies(ctrl, appStack)

			got, err := appStack.ResourceTemplate(tc.given)

			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedTemplate, got)
		})
	}
}

func TestAppParameters(t *testing.T) {
	expectedParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(appAdminRoleParamName),
			ParameterValue: aws.String("testapp-adminrole"),
		},
		{
			ParameterKey:   aws.String(appExecutionRoleParamName),
			ParameterValue: aws.String("testapp-executionrole"),
		},
		{
			ParameterKey:   aws.String(appDNSDelegatedAccountsKey),
			ParameterValue: aws.String("1234"),
		},
		{
			ParameterKey:   aws.String(appDomainNameKey),
			ParameterValue: aws.String("amazon.com"),
		},
		{
			ParameterKey:   aws.String(appDomainHostedZoneIDKey),
			ParameterValue: aws.String("mockHostedZoneID"),
		},
		{
			ParameterKey:   aws.String(appDNSDelegationRoleParamName),
			ParameterValue: aws.String("testapp-DNSDelegationRole"),
		},
		{
			ParameterKey:   aws.String(appNameKey),
			ParameterValue: aws.String("testapp"),
		},
	}
	app := &AppStackConfig{
		CreateAppInput: &deploy.CreateAppInput{Name: "testapp", AccountID: "1234", DomainName: "amazon.com", DomainHostedZoneID: "mockHostedZoneID"},
	}
	params, _ := app.Parameters()
	require.ElementsMatch(t, expectedParams, params)
}

func TestAppTags(t *testing.T) {
	app := &AppStackConfig{
		CreateAppInput: &deploy.CreateAppInput{
			Name:      "testapp",
			AccountID: "1234",
			AdditionalTags: map[string]string{
				"confidentiality": "public",
				"owner":           "finance",
				deploy.AppTagKey:  "overrideapp",
			},
		},
	}
	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(deploy.AppTagKey),
			Value: aws.String(app.Name),
		},
		{
			Key:   aws.String("confidentiality"),
			Value: aws.String("public"),
		},
		{
			Key:   aws.String("owner"),
			Value: aws.String("finance"),
		},
	}
	require.ElementsMatch(t, expectedTags, app.Tags())
}

func TestToRegionalResources(t *testing.T) {
	testCases := map[string]struct {
		givenStackOutputs map[string]string
		wantedResource    AppRegionalResources
		wantedErr         error
	}{
		"should generate fully formed resource": {
			givenStackOutputs: map[string]string{
				appOutputKMSKey:       "arn:aws:kms:us-west-2:01234567890:key/0000",
				appOutputS3Bucket:     "tests3-bucket-us-west-2",
				"ECRRepofrontDASHend": "arn:aws:ecr:us-west-2:0123456789:repository/app/front-end",
				"ECRRepobackDASHend":  "arn:aws:ecr:us-west-2:0123456789:repository/app/back-end",
			},
			wantedResource: AppRegionalResources{
				KMSKeyARN: "arn:aws:kms:us-west-2:01234567890:key/0000",
				S3Bucket:  "tests3-bucket-us-west-2",
				RepositoryURLs: map[string]string{
					"front-end": "0123456789.dkr.ecr.us-west-2.amazonaws.com/app/front-end",
					"back-end":  "0123456789.dkr.ecr.us-west-2.amazonaws.com/app/back-end",
				},
			},
		},
		"should return error when no bucket exists": {
			givenStackOutputs: map[string]string{
				appOutputKMSKey:       "arn:aws:kms:us-west-2:01234567890:key/0000",
				"ECRRepofrontDASHend": "arn:aws:ecr:us-west-2:0123456789:repository/app/front-end",
				"ECRRepobackDASHend":  "arn:aws:ecr:us-west-2:0123456789:repository/app/back-end",
			},
			wantedErr: fmt.Errorf("couldn't find S3 bucket output key PipelineBucket in stack stack"),
		},
		"should return error when no kms key exists": {
			givenStackOutputs: map[string]string{
				appOutputS3Bucket:     "tests3-bucket-us-west-2",
				"ECRRepofrontDASHend": "arn:aws:ecr:us-west-2:0123456789:repository/app/front-end",
				"ECRRepobackDASHend":  "arn:aws:ecr:us-west-2:0123456789:repository/app/back-end",
			},
			wantedErr: fmt.Errorf("couldn't find KMS output key KMSKeyARN in stack stack"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ToAppRegionalResources(mockAppResourceStack("stack", tc.givenStackOutputs))

			if tc.wantedErr != nil {
				require.EqualError(t, tc.wantedErr, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedResource, *got)
			}
		})
	}
}

func TestDNSDelegatedAccountsForStack(t *testing.T) {
	testCases := map[string]struct {
		given map[string]string
		want  []string
	}{
		"should read from parameter and parse comma seperated list": {
			given: map[string]string{
				appDNSDelegatedAccountsKey: "1234,5678",
			},
			want: []string{"1234", "5678"},
		},
		"should return empty when no field is found": {
			given: map[string]string{
				"not a real field": "ok",
			},
			want: []string{},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := DNSDelegatedAccountsForStack(mockAppRolesStack("stack", tc.given))
			require.ElementsMatch(t, tc.want, got)
		})
	}
}

func mockAppResourceStack(stackArn string, outputs map[string]string) *cloudformation.Stack {
	outputList := []*cloudformation.Output{}
	for key, val := range outputs {
		outputList = append(outputList, &cloudformation.Output{
			OutputKey:   aws.String(key),
			OutputValue: aws.String(val),
		})
	}

	return &cloudformation.Stack{
		StackId: aws.String(stackArn),
		Outputs: outputList,
	}
}

func mockAppRolesStack(stackArn string, parameters map[string]string) *cloudformation.Stack {
	parameterList := []*cloudformation.Parameter{}
	for key, val := range parameters {
		parameterList = append(parameterList, &cloudformation.Parameter{
			ParameterKey:   aws.String(key),
			ParameterValue: aws.String(val),
		})
	}

	return &cloudformation.Stack{
		StackId:    aws.String(stackArn),
		Parameters: parameterList,
	}
}

func TestAppStackName(t *testing.T) {
	app := &AppStackConfig{
		CreateAppInput: &deploy.CreateAppInput{Name: "testapp", AccountID: "1234"},
	}
	require.Equal(t, fmt.Sprintf("%s-infrastructure-roles", app.Name), app.StackName())
}

func TestAppStackSetName(t *testing.T) {
	app := &AppStackConfig{
		CreateAppInput: &deploy.CreateAppInput{Name: "testapp", AccountID: "1234"},
	}
	require.Equal(t, fmt.Sprintf("%s-infrastructure", app.Name), app.StackSetName())
}

func TestTemplateToAppConfig(t *testing.T) {
	given := `AWSTemplateFormatVersion: '2010-09-09'
Description: Cross-regional resources to support the CodePipeline for a workspace
Metadata:
  Version: 7
  Services:
  - testsvc1
  - testsvc2
  Accounts:
  - 0000000000
`
	config, err := AppConfigFrom(&given)
	require.NoError(t, err)
	require.Equal(t, AppResourcesConfig{
		Accounts: []string{"0000000000"},
		Version:  7,
		Workloads: []AppResourcesWorkload{
			{Name: "testsvc1", WithECR: true},
			{Name: "testsvc2", WithECR: true},
		},
	}, *config)
}

func TestAppResourcesService_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		in          []byte
		wanted      AppResourcesConfig
		wantedError error
	}{
		"unmarshal legacy service config": {
			in: []byte(`Services:
  - frontend
  - backend
TemplateVersion: 'v1.1.0'
Version: 6
App: demo
Accounts:
  - 1234567890`),
			wanted: AppResourcesConfig{
				Workloads: []AppResourcesWorkload{
					{Name: "frontend", WithECR: true},
					{Name: "backend", WithECR: true},
				},
				Accounts: []string{"1234567890"},
				Version:  6,
				App:      "demo",
			},
		},
		"unmarshal v1.28, v1.29 service config field": {
			in: []byte(`Workloads:
  - Name: frontend
    WithECR: true
  - Name: backend
    WithECR: false
TemplateVersion: 'v1.1.0'
Version: 6
App: demo
Accounts:
  - 1234567890`),
			wanted: AppResourcesConfig{
				Workloads: []AppResourcesWorkload{
					{Name: "frontend", WithECR: true},
					{Name: "backend", WithECR: false},
				},
				Accounts: []string{"1234567890"},
				Version:  6,
				App:      "demo",
			},
		},
		"unmarshal new service config": {
			in: []byte(`Workloads:
  - Name: frontend
    WithECR: true
  - Name: backend
    WithECR: false
TemplateVersion: 'v1.1.0'
Services: "See #5140"
Version: 6
App: demo
Accounts:
  - 1234567890`),
			wanted: AppResourcesConfig{
				Workloads: []AppResourcesWorkload{
					{Name: "frontend", WithECR: true},
					{Name: "backend", WithECR: false},
				},
				Accounts: []string{"1234567890"},
				Version:  6,
				App:      "demo",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var s AppResources
			err := yaml.Unmarshal(tc.in, &s)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, s.AppResourcesConfig)
			}
		})
	}
}
