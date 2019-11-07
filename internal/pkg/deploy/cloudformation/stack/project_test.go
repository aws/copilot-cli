// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

const (
	mockTemplate = "mockTemplate"
)

func TestProjTemplate(t *testing.T) {
	testCases := map[string]struct {
		box            packd.Box
		expectedOutput string
		want           error
	}{
		"should return error given template not found": {
			box:  emptyProjectBox(),
			want: fmt.Errorf("failed to find the cloudformation template at %s", projectTemplatePath),
		},
		"should return template body when present": {
			box:            projectBoxWithTemplateFile(),
			expectedOutput: mockTemplate,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			projStack := NewProjectStackConfig(&deploy.CreateProjectInput{Project: "testproject", AccountID: "1234"}, tc.box)
			got, err := projStack.Template()

			if tc.want != nil {
				require.EqualError(t, tc.want, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedOutput, got)
			}
		})
	}
}

func TestProjResourceTemplate(t *testing.T) {
	properlyEscapedTemplate := `AWSTemplateFormatVersion: '2010-09-09'
Outputs:
  KMSKeyARN:
    Description: KMS Key used by CodePipeline for encrypting artifacts.
    Value: !GetAtt KMSKey.Arn
  PipelineBucket:
    Description: Bucket used for CodePipeline to stage resources in.
    Value: !Ref PipelineBuiltArtifactBucket
  ECRRepoappDASH1:
    Description: ECR Repo used to store images of the app-1 app.
	Value: !GetAtt ECRRepoappDASH1.Arn`

	testCases := map[string]struct {
		box            packd.Box
		expectedOutput string
		given          *ProjectResourcesConfig
		want           error
	}{
		"should return error given template not found": {
			box:   emptyProjectBox(),
			given: &ProjectResourcesConfig{},
			want:  fmt.Errorf("failed to find the cloudformation template at %s", projectResourcesTemplatePath),
		},
		"should return template body when present": {
			box:            projectBoxWithTemplateFile(),
			given:          &ProjectResourcesConfig{},
			expectedOutput: mockTemplate,
		},
		"should replace dashes in logical IDs": {
			box: projectBoxTemplateFileWithSafeLogicalIDs(),
			given: &ProjectResourcesConfig{
				Accounts: []string{"1234"},
				Apps:     []string{"app-1"},
				Version:  1,
				Project:  "testproject"},
			expectedOutput: properlyEscapedTemplate,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			projStack := NewProjectStackConfig(&deploy.CreateProjectInput{Project: "testproject", AccountID: "1234"}, tc.box)

			got, err := projStack.ResourceTemplate(tc.given)

			if tc.want != nil {
				require.EqualError(t, tc.want, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedOutput, got)
			}
		})
	}
}

func TestProjectParameters(t *testing.T) {
	proj := NewProjectStackConfig(&deploy.CreateProjectInput{Project: "testproject", AccountID: "1234"}, emptyProjectBox())
	expectedParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(projectAdminRoleParamName),
			ParameterValue: aws.String("testproject-adminrole"),
		},
		{
			ParameterKey:   aws.String(projectExecutionRoleParamName),
			ParameterValue: aws.String("testproject-executionrole"),
		},
	}
	require.ElementsMatch(t, expectedParams, proj.Parameters())
}

func TestProjectTags(t *testing.T) {
	proj := NewProjectStackConfig(&deploy.CreateProjectInput{Project: "testproject", AccountID: "1234"}, emptyProjectBox())
	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(projectTagKey),
			Value: aws.String(proj.Project),
		},
	}
	require.ElementsMatch(t, expectedTags, proj.Tags())
}

func TestProjectStackName(t *testing.T) {
	proj := NewProjectStackConfig(&deploy.CreateProjectInput{Project: "testproject", AccountID: "1234"}, emptyProjectBox())
	require.Equal(t, fmt.Sprintf("%s-infrastructure-roles", proj.Project), proj.StackName())
}

func TestProjectStackSetName(t *testing.T) {
	proj := NewProjectStackConfig(&deploy.CreateProjectInput{Project: "testproject", AccountID: "1234"}, emptyProjectBox())
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

func projectBoxWithTemplateFile() packd.Box {
	box := packd.NewMemoryBox()

	box.AddString(projectTemplatePath, mockTemplate)
	box.AddString(projectResourcesTemplatePath, mockTemplate)
	return box
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
