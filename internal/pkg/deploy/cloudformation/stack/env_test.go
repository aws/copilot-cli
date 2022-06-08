// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnv_Template(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, e *EnvStackConfig)
		expectedOutput   string
		want             error
	}{
		"should return template body when present": {
			mockDependencies: func(ctrl *gomock.Controller, e *EnvStackConfig) {
				m := mocks.NewMockenvReadParser(ctrl)
				m.EXPECT().ParseEnv(&template.EnvOpts{
					AppName:                "project",
					ScriptBucketName:       "mockbucket",
					DNSCertValidatorLambda: "mockkey1",
					DNSDelegationLambda:    "mockkey2",
					CustomDomainLambda:     "mockkey4",
					VPCConfig: template.VPCConfig{
						Imported: &template.ImportVPC{},
						Managed: template.ManagedVPC{
							CIDR:               DefaultVPCCIDR,
							PrivateSubnetCIDRs: DefaultPrivateSubnetCIDRs,
							PublicSubnetCIDRs:  DefaultPublicSubnetCIDRs,
						},
					},
					LatestVersion: deploy.LatestEnvTemplateVersion,
				}, gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("mockTemplate")}, nil)
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
				in: mockDeployEnvironmentInput(),
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

func TestEnv_Parameters(t *testing.T) {
	deploymentInput := mockDeployEnvironmentInput()
	deploymentInputWithDNS := mockDeployEnvironmentInput()
	deploymentInputWithDNS.App.Domain = "ecs.aws"
	deploymentInputWithPrivateDNS := mockDeployEnvironmentInput()
	deploymentInputWithPrivateDNS.ImportCertARNs = []string{"arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"}
	deploymentInputWithIngressAllowed := mockDeployEnvironmentInput()
	deploymentInputWithIngressAllowed.AllowVPCIngress = true
	testCases := map[string]struct {
		input *deploy.CreateEnvironmentInput
		want  []*cloudformation.Parameter
	}{
		"without DNS": {
			input: deploymentInput,
			want: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(envParamAppNameKey),
					ParameterValue: aws.String(deploymentInput.App.Name),
				},
				{
					ParameterKey:   aws.String(envParamEnvNameKey),
					ParameterValue: aws.String(deploymentInput.Name),
				},
				{
					ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
					ParameterValue: aws.String(deploymentInput.App.AccountPrincipalARN),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamAliasesKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamInternalALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamAllowVPCIngressKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamEFSWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamNATWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
					ParameterValue: aws.String("env.project.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
			},
		},
		"with DNS": {
			input: deploymentInputWithDNS,
			want: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(envParamAppNameKey),
					ParameterValue: aws.String(deploymentInputWithDNS.App.Name),
				},
				{
					ParameterKey:   aws.String(envParamEnvNameKey),
					ParameterValue: aws.String(deploymentInputWithDNS.Name),
				},
				{
					ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
					ParameterValue: aws.String(deploymentInputWithDNS.App.AccountPrincipalARN),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSKey),
					ParameterValue: aws.String(deploymentInputWithDNS.App.Domain),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
					ParameterValue: aws.String("arn:aws:iam::000000000:role/project-DNSDelegationRole"),
				},
				{
					ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
					ParameterValue: aws.String("env.project.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("true"),
				},
				{
					ParameterKey:   aws.String(EnvParamAliasesKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamInternalALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamAllowVPCIngressKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamEFSWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamNATWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
			},
		},
		"with private DNS": {
			input: deploymentInputWithPrivateDNS,
			want: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(envParamAppNameKey),
					ParameterValue: aws.String(deploymentInput.App.Name),
				},
				{
					ParameterKey:   aws.String(envParamEnvNameKey),
					ParameterValue: aws.String(deploymentInput.Name),
				},
				{
					ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
					ParameterValue: aws.String(deploymentInput.App.AccountPrincipalARN),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamAliasesKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamInternalALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamAllowVPCIngressKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamEFSWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamNATWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
					ParameterValue: aws.String("env.project.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("true"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("true"),
				},
			},
		},
		"with internal ALB ingress allowed": {
			input: deploymentInputWithIngressAllowed,
			want: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(envParamAppNameKey),
					ParameterValue: aws.String(deploymentInput.App.Name),
				},
				{
					ParameterKey:   aws.String(envParamEnvNameKey),
					ParameterValue: aws.String(deploymentInput.Name),
				},
				{
					ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
					ParameterValue: aws.String(deploymentInput.App.AccountPrincipalARN),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamAliasesKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamInternalALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamAllowVPCIngressKey),
					ParameterValue: aws.String("true"),
				},
				{
					ParameterKey:   aws.String(envParamEFSWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamNATWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
					ParameterValue: aws.String("env.project.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("true"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			env := &EnvStackConfig{
				in: tc.input,
			}
			params, _ := env.Parameters()
			require.ElementsMatch(t, tc.want, params)
		})
	}
}

func TestEnv_Tags(t *testing.T) {
	env := &EnvStackConfig{
		in: &deploy.CreateEnvironmentInput{
			Name: "env",
			App: deploy.AppInformation{
				Name: "project",
			},
			AdditionalTags: map[string]string{
				"owner":          "boss",
				deploy.AppTagKey: "overrideproject",
			},
		},
	}
	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(deploy.AppTagKey),
			Value: aws.String("project"), // Ignore user's overrides.
		},
		{
			Key:   aws.String(deploy.EnvTagKey),
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
		in: deploymentInput,
	}
	require.Equal(t, fmt.Sprintf("%s-%s", deploymentInput.App.Name, deploymentInput.Name), env.StackName())
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
				App:              mockDeployInput.App.Name,
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
				in: mockDeployInput,
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
				OutputKey:   aws.String(envOutputManagerRoleKey),
				OutputValue: aws.String(managerRoleARN),
			},
			{
				OutputKey:   aws.String(envOutputCFNExecutionRoleARN),
				OutputValue: aws.String(executionRoleARN),
			},
		},
	}
}

func mockDeployEnvironmentInput() *deploy.CreateEnvironmentInput {
	return &deploy.CreateEnvironmentInput{
		Name: "env",
		App: deploy.AppInformation{
			Name:                "project",
			AccountPrincipalARN: "arn:aws:iam::000000000:root",
		},
		CustomResourcesURLs: map[string]string{
			template.DNSCertValidatorFileName: "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey1",
			template.DNSDelegationFileName:    "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey2",
			template.CustomDomainFileName:     "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey4",
		},
		ImportVPCConfig: &config.ImportVPC{},
	}
}
