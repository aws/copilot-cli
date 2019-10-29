// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

func TestEnvTemplate(t *testing.T) {
	testCases := map[string]struct {
		box            packd.Box
		expectedOutput string
		want           error
	}{
		"should return error given template not found": {
			box:  emptyEnvBox(),
			want: fmt.Errorf("failed to find the cloudformation template at %s", envTemplatePath),
		},
		"should return template body when present": {
			box:            envBoxWithTemplateFile(),
			expectedOutput: mockTemplate,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			envStack := newEnvStackConfig(mockDeployEnvironmentInput(), tc.box)
			got, err := envStack.Template()

			if tc.want != nil {
				require.EqualError(t, tc.want, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedOutput, got)
			}
		})
	}
}

func TestEnvParameters(t *testing.T) {
	deploymentInput := mockDeployEnvironmentInput()
	env := newEnvStackConfig(deploymentInput, emptyEnvBox())
	expectedParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(EnvParamIncludeLBKey),
			ParameterValue: aws.String(strconv.FormatBool(deploymentInput.PublicLoadBalancer)),
		},
		{
			ParameterKey:   aws.String(EnvParamProjectNameKey),
			ParameterValue: aws.String(deploymentInput.Project),
		},
		{
			ParameterKey:   aws.String(EnvParamEnvNameKey),
			ParameterValue: aws.String(deploymentInput.Name),
		},
		{
			ParameterKey:   aws.String(envParamToolsAccountPrincipal),
			ParameterValue: aws.String(deploymentInput.ToolsAccountPrincipalARN),
		},
	}
	require.ElementsMatch(t, expectedParams, env.Parameters())
}

func TestEnvTags(t *testing.T) {
	deploymentInput := mockDeployEnvironmentInput()
	env := newEnvStackConfig(deploymentInput, emptyEnvBox())
	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(projectTagKey),
			Value: aws.String(deploymentInput.Project),
		},
		{
			Key:   aws.String(envTagKey),
			Value: aws.String(deploymentInput.Name),
		},
	}
	require.ElementsMatch(t, expectedTags, env.Tags())
}

func TestStackName(t *testing.T) {
	deploymentInput := mockDeployEnvironmentInput()
	env := newEnvStackConfig(deploymentInput, emptyEnvBox())
	require.Equal(t, fmt.Sprintf("%s-%s", deploymentInput.Project, deploymentInput.Name), env.StackName())
}

func TestToEnv(t *testing.T) {
	mockDeployInput := mockDeployEnvironmentInput()
	testCases := map[string]struct {
		expectedEnv archer.Environment
		mockStack   *cloudformation.Stack
		want        error
	}{
		"should return error if Stack ID is invalid": {
			want:      fmt.Errorf("couldn't extract region and account from stack ID : arn: invalid prefix"),
			mockStack: mockEnvironmentStack("", ""),
		},
		"should return a well formed environment": {
			mockStack: mockEnvironmentStack("arn:aws:cloudformation:eu-west-3:902697171733:stack/project-env", "project/env"),
			expectedEnv: archer.Environment{
				Name:        mockDeployInput.Name,
				Project:     mockDeployInput.Project,
				Prod:        mockDeployInput.Prod,
				AccountID:   "902697171733",
				Region:      "eu-west-3",
				RegistryURL: "902697171733.dkr.ecr.eu-west-3.amazonaws.com/project/env",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			envStack := newEnvStackConfig(mockDeployInput, emptyBox())
			got, err := envStack.ToEnv(tc.mockStack)

			if tc.want != nil {
				require.EqualError(t, tc.want, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedEnv, *got)
			}
		})
	}
}
func mockEnvironmentStack(stackArn, ecrOutput string) *cloudformation.Stack {
	return &cloudformation.Stack{
		StackId: aws.String(stackArn),
		Outputs: []*cloudformation.Output{
			&cloudformation.Output{
				OutputKey:   aws.String(EnvOutputECRKey),
				OutputValue: aws.String(ecrOutput),
			},
		},
	}
}

func mockDeployEnvironmentInput() *deploy.CreateEnvironmentInput {
	return &deploy.CreateEnvironmentInput{
		Name:                     "env",
		Project:                  "project",
		Prod:                     true,
		PublicLoadBalancer:       true,
		ToolsAccountPrincipalARN: "mockToolsAccountPrincipalARN",
	}
}
func emptyEnvBox() packd.Box {
	return packd.NewMemoryBox()
}

func envBoxWithTemplateFile() packd.Box {
	box := packd.NewMemoryBox()

	box.AddString(envTemplatePath, mockTemplate)

	return box
}
