// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnvTemplate(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, e *EnvStackConfig)
		expectedOutput   string
		want             error
	}{
		"should return error given template not found": {
			mockDependencies: func(ctrl *gomock.Controller, e *EnvStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Read(dnsDelegationTemplatePath).Return(nil, errors.New("some error"))
				e.parser = m
			},
			want: errors.New("some error"),
		},
		"should return template body when present": {
			mockDependencies: func(ctrl *gomock.Controller, e *EnvStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Read(dnsDelegationTemplatePath).Return(&template.Content{Buffer: bytes.NewBufferString("customresources")}, nil)
				m.EXPECT().Read(acmValidationTemplatePath).Return(&template.Content{Buffer: bytes.NewBufferString("customresources")}, nil)
				m.EXPECT().Read(enableLongARNsTemplatePath).Return(&template.Content{Buffer: bytes.NewBufferString("customresources")}, nil)
				m.EXPECT().Parse(EnvTemplatePath, struct {
					DNSDelegationLambda       string
					ACMValidationLambda       string
					EnableLongARNFormatLambda string
				}{
					"customresources",
					"customresources",
					"customresources",
				}).Return(&template.Content{Buffer: bytes.NewBufferString("mockTemplate")}, nil)
				e.parser = m
			},
			expectedOutput: mockTemplate,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			envStack := &EnvStackConfig{
				CreateEnvironmentInput: mockDeployEnvironmentInput(),
			}
			tc.mockDependencies(ctrl, envStack)

			// WHEN
			got, err := envStack.Template()

			// THEN
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
	deploymentInputWithDNS.AppDNSName = "ecs.aws"
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
					ParameterKey:   aws.String(envParamAppNameKey),
					ParameterValue: aws.String(deploymentInput.AppName),
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
					ParameterKey:   aws.String(envParamAppDNSKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
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
					ParameterKey:   aws.String(envParamAppNameKey),
					ParameterValue: aws.String(deploymentInputWithDNS.AppName),
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
					ParameterKey:   aws.String(envParamAppDNSKey),
					ParameterValue: aws.String(deploymentInputWithDNS.AppDNSName),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
					ParameterValue: aws.String("arn:aws:iam::000000000:role/project-DNSDelegationRole"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			env := &EnvStackConfig{
				CreateEnvironmentInput: tc.input,
			}
			params, _ := env.Parameters()
			require.ElementsMatch(t, tc.want, params)
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
					AppDNSName:               "ecs.aws",
				},
			},
		},
		"without DNS": {
			want: "",
			input: &EnvStackConfig{
				CreateEnvironmentInput: &deploy.CreateEnvironmentInput{
					ToolsAccountPrincipalARN: "arn:aws:iam::0000000:root",
					AppDNSName:               "",
				},
			},
		},
		"with invalid tools principal": {
			want: "",
			input: &EnvStackConfig{
				CreateEnvironmentInput: &deploy.CreateEnvironmentInput{
					ToolsAccountPrincipalARN: "0000000",
					AppDNSName:               "ecs.aws",
				},
			},
		},
		"with dns and tools principal": {
			want: "arn:aws:iam::0000000:role/-DNSDelegationRole",
			input: &EnvStackConfig{
				CreateEnvironmentInput: &deploy.CreateEnvironmentInput{
					ToolsAccountPrincipalARN: "arn:aws:iam::0000000:root",
					AppDNSName:               "ecs.aws",
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
	env := &EnvStackConfig{
		CreateEnvironmentInput: &deploy.CreateEnvironmentInput{
			Name:    "env",
			AppName: "project",
			AdditionalTags: map[string]string{
				"owner":   "boss",
				AppTagKey: "overrideproject",
			},
		},
	}
	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(AppTagKey),
			Value: aws.String("project"), // Ignore user's overrides.
		},
		{
			Key:   aws.String(EnvTagKey),
			Value: aws.String("env"),
		},
		{
			Key:   aws.String("owner"),
			Value: aws.String("boss"),
		},
	}
	require.ElementsMatch(t, expectedTags, env.Tags())
}

func TestStackName(t *testing.T) {
	deploymentInput := mockDeployEnvironmentInput()
	env := &EnvStackConfig{
		CreateEnvironmentInput: deploymentInput,
	}
	require.Equal(t, fmt.Sprintf("%s-%s", deploymentInput.AppName, deploymentInput.Name), env.StackName())
}

func TestToEnv(t *testing.T) {
	mockDeployInput := mockDeployEnvironmentInput()
	testCases := map[string]struct {
		expectedEnv config.Environment
		mockStack   *cloudformation.Stack
		want        error
	}{
		"should return error if Stack ID is invalid": {
			want:      fmt.Errorf("couldn't extract region and account from stack ID : arn: invalid prefix"),
			mockStack: mockEnvironmentStack("", "", ""),
		},
		"should return a well formed environment": {
			mockStack: mockEnvironmentStack(
				"arn:aws:cloudformation:eu-west-3:902697171733:stack/project-env",
				"arn:aws:iam::902697171733:role/phonetool-test-EnvManagerRole",
				"arn:aws:iam::902697171733:role/phonetool-test-CFNExecutionRole"),
			expectedEnv: config.Environment{
				Name:             mockDeployInput.Name,
				App:              mockDeployInput.AppName,
				Prod:             mockDeployInput.Prod,
				AccountID:        "902697171733",
				Region:           "eu-west-3",
				ManagerRoleARN:   "arn:aws:iam::902697171733:role/phonetool-test-EnvManagerRole",
				ExecutionRoleARN: "arn:aws:iam::902697171733:role/phonetool-test-CFNExecutionRole",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			envStack := &EnvStackConfig{
				CreateEnvironmentInput: mockDeployInput,
			}
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

func mockEnvironmentStack(stackArn, managerRoleARN, executionRoleARN string) *cloudformation.Stack {
	return &cloudformation.Stack{
		StackId: aws.String(stackArn),
		Outputs: []*cloudformation.Output{
			{
				OutputKey:   aws.String(EnvOutputManagerRoleKey),
				OutputValue: aws.String(managerRoleARN),
			},
			{
				OutputKey:   aws.String(EnvOutputCFNExecutionRoleARN),
				OutputValue: aws.String(executionRoleARN),
			},
		},
	}
}

func mockDeployEnvironmentInput() *deploy.CreateEnvironmentInput {
	return &deploy.CreateEnvironmentInput{
		Name:                     "env",
		AppName:                  "project",
		Prod:                     true,
		PublicLoadBalancer:       true,
		ToolsAccountPrincipalARN: "arn:aws:iam::000000000:root",
	}
}
