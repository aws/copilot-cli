// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

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
			want: fmt.Errorf("failed to find the cloudformation template at %s", EnvTemplatePath),
		},
		"should return template body when present": {
			box:            envBoxWithAllTemplateFiles(),
			expectedOutput: mockTemplate,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			envStack := NewEnvStackConfig(mockDeployEnvironmentInput(), tc.box)
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
	deploymentInputWithDNS := mockDeployEnvironmentInput()
	deploymentInputWithDNS.ProjectDNSName = "ecs.aws"
	testCases := map[string]struct {
		input *deploy.CreateEnvironmentInput
		want  []*cloudformation.Parameter
	}{
		"without DNS": {
			input: deploymentInput,
			want: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(envParamIncludeLBKey),
					ParameterValue: aws.String(strconv.FormatBool(deploymentInput.PublicLoadBalancer)),
				},
				{
					ParameterKey:   aws.String(envParamProjectNameKey),
					ParameterValue: aws.String(deploymentInput.Project),
				},
				{
					ParameterKey:   aws.String(envParamEnvNameKey),
					ParameterValue: aws.String(deploymentInput.Name),
				},
				{
					ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
					ParameterValue: aws.String(deploymentInput.ToolsAccountPrincipalARN),
				},
				{
					ParameterKey:   aws.String(envParamProjectDNSKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamProjectDNSDelegationRoleKey),
					ParameterValue: aws.String(""),
				},
			},
		},
		"with DNS": {
			input: deploymentInputWithDNS,
			want: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(envParamIncludeLBKey),
					ParameterValue: aws.String(strconv.FormatBool(deploymentInputWithDNS.PublicLoadBalancer)),
				},
				{
					ParameterKey:   aws.String(envParamProjectNameKey),
					ParameterValue: aws.String(deploymentInputWithDNS.Project),
				},
				{
					ParameterKey:   aws.String(envParamEnvNameKey),
					ParameterValue: aws.String(deploymentInputWithDNS.Name),
				},
				{
					ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
					ParameterValue: aws.String(deploymentInputWithDNS.ToolsAccountPrincipalARN),
				},
				{
					ParameterKey:   aws.String(envParamProjectDNSKey),
					ParameterValue: aws.String(deploymentInputWithDNS.ProjectDNSName),
				},
				{
					ParameterKey:   aws.String(envParamProjectDNSDelegationRoleKey),
					ParameterValue: aws.String("arn:aws:iam::000000000:role/project-DNSDelegationRole"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			env := NewEnvStackConfig(tc.input, emptyEnvBox())
			require.ElementsMatch(t, tc.want, env.Parameters())
		})
	}
}

func TestEnvDNSDelegationRole(t *testing.T) {
	testCases := map[string]struct {
		input *EnvStackConfig
		want  string
	}{
		"without tools account ARN": {
			want: "",
			input: &EnvStackConfig{
				CreateEnvironmentInput: &deploy.CreateEnvironmentInput{
					ToolsAccountPrincipalARN: "",
					ProjectDNSName:           "ecs.aws",
				},
			},
		},
		"without DNS": {
			want: "",
			input: &EnvStackConfig{
				CreateEnvironmentInput: &deploy.CreateEnvironmentInput{
					ToolsAccountPrincipalARN: "arn:aws:iam::0000000:root",
					ProjectDNSName:           "",
				},
			},
		},
		"with invalid tools principal": {
			want: "",
			input: &EnvStackConfig{
				CreateEnvironmentInput: &deploy.CreateEnvironmentInput{
					ToolsAccountPrincipalARN: "0000000",
					ProjectDNSName:           "ecs.aws",
				},
			},
		},
		"with dns and tools principal": {
			want: "arn:aws:iam::0000000:role/-DNSDelegationRole",
			input: &EnvStackConfig{
				CreateEnvironmentInput: &deploy.CreateEnvironmentInput{
					ToolsAccountPrincipalARN: "arn:aws:iam::0000000:root",
					ProjectDNSName:           "ecs.aws",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.input.dnsDelegationRole())
		})
	}
}

func TestEnvTags(t *testing.T) {
	deploymentInput := mockDeployEnvironmentInput()
	env := NewEnvStackConfig(deploymentInput, emptyEnvBox())
	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(ProjectTagKey),
			Value: aws.String(deploymentInput.Project),
		},
		{
			Key:   aws.String(EnvTagKey),
			Value: aws.String(deploymentInput.Name),
		},
	}
	require.ElementsMatch(t, expectedTags, env.Tags())
}

func TestStackName(t *testing.T) {
	deploymentInput := mockDeployEnvironmentInput()
	env := NewEnvStackConfig(deploymentInput, emptyEnvBox())
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
			mockStack: mockEnvironmentStack("arn:aws:cloudformation:eu-west-3:902697171733:stack/project-env", "arn:aws:iam::902697171733:role/phonetool-test-EnvManagerRole"),
			expectedEnv: archer.Environment{
				Name:           mockDeployInput.Name,
				Project:        mockDeployInput.Project,
				Prod:           mockDeployInput.Prod,
				AccountID:      "902697171733",
				Region:         "eu-west-3",
				ManagerRoleARN: "arn:aws:iam::902697171733:role/phonetool-test-EnvManagerRole",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			envStack := NewEnvStackConfig(mockDeployInput, emptyEnvBox())
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

func TestEnvStack_ExecutionRoleARN(t *testing.T) {
	// GIVEN
	rawStack := &cloudformation.Stack{
		Outputs: []*cloudformation.Output{
			{
				OutputKey:   aws.String(envOutputCFNExecutionRoleARN),
				OutputValue: aws.String("1234"),
			},
		},
	}
	envStack := &EnvStack{Stack: rawStack}

	// WHEN
	got := envStack.ExecutionRoleARN()

	// THEN
	require.Equal(t, "1234", got)
}

func mockEnvironmentStack(stackArn, managerRoleARN string) *cloudformation.Stack {
	return &cloudformation.Stack{
		StackId: aws.String(stackArn),
		Outputs: []*cloudformation.Output{
			{
				OutputKey:   aws.String(envOutputManagerRoleKey),
				OutputValue: aws.String(managerRoleARN),
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
		ToolsAccountPrincipalARN: "arn:aws:iam::000000000:root",
	}
}

func emptyEnvBox() packd.Box {
	return packd.NewMemoryBox()
}

func envBoxWithAllTemplateFiles() packd.Box {
	box := packd.NewMemoryBox()

	box.AddString(EnvTemplatePath, mockTemplate)
	box.AddString(acmValidationTemplatePath, "customresources")
	box.AddString(dnsDelegationTemplatePath, "customresources")

	return box
}
