// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/gobuffalo/packd"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	mockTemplate = "mockTemplate"
)

func TestProjTemplate(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, c *ProjectStackConfig)

		wantedTemplate string
		wantedError    error
	}{
		"should return error given template not found": {
			mockDependencies: func(ctrl *gomock.Controller, c *ProjectStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Read(projectTemplatePath).Return(nil, errors.New("some error"))
				c.parser = m
			},

			wantedError: errors.New("some error"),
		},
		"should return template body when present": {
			mockDependencies: func(ctrl *gomock.Controller, c *ProjectStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Read(projectTemplatePath).Return(&template.Content{
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
			projStack := &ProjectStackConfig{}
			tc.mockDependencies(ctrl, projStack)

			// WHEN
			got, err := projStack.Template()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedTemplate, got)
		})
	}
}

func TestDNSDelegationAccounts(t *testing.T) {
	testCases := map[string]struct {
		given *deploy.CreateProjectInput
		want  []string
	}{
		"should append project account": {
			given: &deploy.CreateProjectInput{
				AccountID: "1234",
			},
			want: []string{"1234"},
		},
		"should ignore duplicates": {
			given: &deploy.CreateProjectInput{
				AccountID:             "1234",
				DNSDelegationAccounts: []string{"1234"},
			},
			want: []string{"1234"},
		},
		"should return a set": {
			given: &deploy.CreateProjectInput{
				AccountID:             "1234",
				DNSDelegationAccounts: []string{"4567"},
			},
			want: []string{"1234", "4567"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			projStack := &ProjectStackConfig{
				CreateProjectInput: tc.given,
			}
			got := projStack.dnsDelegationAccounts()
			require.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestProjResourceTemplate(t *testing.T) {
	testCases := map[string]struct {
		given            *ProjectResourcesConfig
		mockDependencies func(ctrl *gomock.Controller, c *ProjectStackConfig)

		wantedTemplate string
		wantedError    error
	}{
		"should return error when template cannot be parsed": {
			given: &ProjectResourcesConfig{},
			mockDependencies: func(ctrl *gomock.Controller, c *ProjectStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				c.parser = m
			},

			wantedError: errors.New("some error"),
		},
		"should render template after sorting": {
			given: &ProjectResourcesConfig{
				Accounts: []string{"4567", "1234"},
				Apps:     []string{"app-2", "app-1"},
				Version:  1,
				Project:  "testproject",
			},
			mockDependencies: func(ctrl *gomock.Controller, c *ProjectStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(projectResourcesTemplatePath, struct {
					*ProjectResourcesConfig
					AppTagKey string
				}{
					&ProjectResourcesConfig{
						Accounts: []string{"1234", "4567"},
						Apps:     []string{"app-1", "app-2"},
						Version:  1,
						Project:  "testproject",
					},
					AppTagKey,
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
			projStack := &ProjectStackConfig{
				CreateProjectInput: &deploy.CreateProjectInput{Project: "testproject", AccountID: "1234"},
			}
			tc.mockDependencies(ctrl, projStack)

			got, err := projStack.ResourceTemplate(tc.given)

			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedTemplate, got)
		})
	}
}

func TestProjectParameters(t *testing.T) {
	expectedParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(projectAdminRoleParamName),
			ParameterValue: aws.String("testproject-adminrole"),
		},
		{
			ParameterKey:   aws.String(projectExecutionRoleParamName),
			ParameterValue: aws.String("testproject-executionrole"),
		},
		{
			ParameterKey:   aws.String(projectDNSDelegatedAccountsKey),
			ParameterValue: aws.String("1234"),
		},
		{
			ParameterKey:   aws.String(projectDomainNameKey),
			ParameterValue: aws.String("amazon.com"),
		},
		{
			ParameterKey:   aws.String(projectDNSDelegationRoleParamName),
			ParameterValue: aws.String("testproject-DNSDelegationRole"),
		},
		{
			ParameterKey:   aws.String(projectNameKey),
			ParameterValue: aws.String("testproject"),
		},
	}
	proj := &ProjectStackConfig{
		CreateProjectInput: &deploy.CreateProjectInput{Project: "testproject", AccountID: "1234", DomainName: "amazon.com"},
	}
	require.ElementsMatch(t, expectedParams, proj.Parameters())
}

func TestProjectTags(t *testing.T) {
	proj := &ProjectStackConfig{
		CreateProjectInput: &deploy.CreateProjectInput{
			Project:   "testproject",
			AccountID: "1234",
			AdditionalTags: map[string]string{
				"confidentiality": "public",
				"owner":           "finance",
				ProjectTagKey:     "overrideproject",
			},
		},
	}
	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(ProjectTagKey),
			Value: aws.String(proj.Project),
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
	require.ElementsMatch(t, expectedTags, proj.Tags())
}

func TestToRegionalResources(t *testing.T) {
	testCases := map[string]struct {
		givenStackOutputs map[string]string
		wantedResource    archer.ProjectRegionalResources
		wantedErr         error
	}{
		"should generate fully formed resource": {
			givenStackOutputs: map[string]string{
				projectOutputKMSKey:   "arn:aws:kms:us-west-2:01234567890:key/0000",
				projectOutputS3Bucket: "tests3-bucket-us-west-2",
				"ECRRepofrontDASHend": "arn:aws:ecr:us-west-2:0123456789:repository/project/front-end",
				"ECRRepobackDASHend":  "arn:aws:ecr:us-west-2:0123456789:repository/project/back-end",
			},
			wantedResource: archer.ProjectRegionalResources{
				KMSKeyARN: "arn:aws:kms:us-west-2:01234567890:key/0000",
				S3Bucket:  "tests3-bucket-us-west-2",
				RepositoryURLs: map[string]string{
					"front-end": "0123456789.dkr.ecr.us-west-2.amazonaws.com/project/front-end",
					"back-end":  "0123456789.dkr.ecr.us-west-2.amazonaws.com/project/back-end",
				},
			},
		},
		"should return error when no bucket exists": {
			givenStackOutputs: map[string]string{
				projectOutputKMSKey:   "arn:aws:kms:us-west-2:01234567890:key/0000",
				"ECRRepofrontDASHend": "arn:aws:ecr:us-west-2:0123456789:repository/project/front-end",
				"ECRRepobackDASHend":  "arn:aws:ecr:us-west-2:0123456789:repository/project/back-end",
			},
			wantedErr: fmt.Errorf("couldn't find S3 bucket output key PipelineBucket in stack stack"),
		},
		"should return error when no kms key exists": {
			givenStackOutputs: map[string]string{
				projectOutputS3Bucket: "tests3-bucket-us-west-2",
				"ECRRepofrontDASHend": "arn:aws:ecr:us-west-2:0123456789:repository/project/front-end",
				"ECRRepobackDASHend":  "arn:aws:ecr:us-west-2:0123456789:repository/project/back-end",
			},
			wantedErr: fmt.Errorf("couldn't find KMS output key KMSKeyARN in stack stack"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ToProjectRegionalResources(mockProjectResourceStack("stack", tc.givenStackOutputs))

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
				projectDNSDelegatedAccountsKey: "1234,5678",
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
			got := DNSDelegatedAccountsForStack(mockProjectRolesStack("stack", tc.given))
			require.ElementsMatch(t, tc.want, got)
		})
	}
}

func mockProjectResourceStack(stackArn string, outputs map[string]string) *cloudformation.Stack {
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

func mockProjectRolesStack(stackArn string, parameters map[string]string) *cloudformation.Stack {
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

func TestProjectStackName(t *testing.T) {
	proj := &ProjectStackConfig{
		CreateProjectInput: &deploy.CreateProjectInput{Project: "testproject", AccountID: "1234"},
	}
	require.Equal(t, fmt.Sprintf("%s-infrastructure-roles", proj.Project), proj.StackName())
}

func TestProjectStackSetName(t *testing.T) {
	proj := &ProjectStackConfig{
		CreateProjectInput: &deploy.CreateProjectInput{Project: "testproject", AccountID: "1234"},
	}
	require.Equal(t, fmt.Sprintf("%s-infrastructure", proj.Project), proj.StackSetName())
}

func TestTemplateToProjectConfig(t *testing.T) {
	given := `AWSTemplateFormatVersion: '2010-09-09'
Description: Cross-regional resources to support the CodePipeline for a workspace
Metadata:
  Version: 7
  Apps:
  - testapp1
  - testapp2
  Accounts:
  - 0000000000
`
	config, err := ProjectConfigFrom(&given)
	require.NoError(t, err)
	require.Equal(t, ProjectResourcesConfig{
		Accounts: []string{"0000000000"},
		Version:  7,
		Apps:     []string{"testapp1", "testapp2"},
	}, *config)
}

func emptyProjectBox() packd.Box {
	return packd.NewMemoryBox()
}

func projectBoxTemplateFileWithSafeLogicalIDs() packd.Box {
	box := packd.NewMemoryBox()
	templateWithFunction := `AWSTemplateFormatVersion: '2010-09-09'
Outputs:
  KMSKeyARN:
    Description: KMS Key used by CodePipeline for encrypting artifacts.
    Value: !GetAtt KMSKey.Arn
  PipelineBucket:
    Description: Bucket used for CodePipeline to stage resources in.
    Value: !Ref PipelineBuiltArtifactBucket
{{range $app := .Apps}}  ECRRepo{{logicalIDSafe $app}}:
    Description: ECR Repo used to store images of the {{$app}} app.
	Value: !GetAtt ECRRepo{{logicalIDSafe $app}}.Arn{{end}}`

	box.AddString(projectTemplatePath, mockTemplate)
	box.AddString(projectResourcesTemplatePath, templateWithFunction)
	return box
}
