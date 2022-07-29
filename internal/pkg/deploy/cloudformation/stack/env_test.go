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
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
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
				m.EXPECT().ParseEnv(gomock.Any()).DoAndReturn(func(data *template.EnvOpts) (*template.Content, error) {
					require.Equal(t, &template.EnvOpts{
						AppName: "project",
						EnvName: "env",
						VPCConfig: template.VPCConfig{
							Imported: nil,
							Managed: template.ManagedVPC{
								CIDR:               DefaultVPCCIDR,
								PrivateSubnetCIDRs: DefaultPrivateSubnetCIDRs,
								PublicSubnetCIDRs:  DefaultPublicSubnetCIDRs,
							},
							SecurityGroupConfig: nil,
						},
						LatestVersion: deploy.LatestEnvTemplateVersion,
						CustomResources: map[string]template.S3ObjectLocation{
							"CertificateValidationFunction": {
								Bucket: "mockbucket",
								Key:    "mockkey1",
							},
							"DNSDelegationFunction": {
								Bucket: "mockbucket",
								Key:    "mockkey2",
							},
							"CustomDomainFunction": {
								Bucket: "mockbucket",
								Key:    "mockkey4",
							},
						},
						Telemetry: &template.Telemetry{
							EnableContainerInsights: false,
						},

						SerializedManifest: "name: env\ntype: Environment\n",
						ForceUpdateID:      "mockPreviousForceUpdateID",
					}, data)
					return &template.Content{Buffer: bytes.NewBufferString("mockTemplate")}, nil
				})
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
				in:                mockDeployEnvironmentInput(),
				lastForceUpdateID: "mockPreviousForceUpdateID",
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
	deploymentInputWithPrivateDNS.Mft.HTTPConfig.Private.Certificates = []string{"arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"}
	testCases := map[string]struct {
		input     *deploy.CreateEnvironmentInput
		oldParams []*cloudformation.Parameter
		want      []*cloudformation.Parameter
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
		"with private DNS only": {
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
					ParameterValue: aws.String("true"),
				},
			},
		},
		"should retain the values from EnvControllerParameters": {
			input: deploymentInput,
			oldParams: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
					ParameterValue: aws.String("frontend,backend"),
				},
				{
					ParameterKey:   aws.String(envParamNATWorkloadsKey),
					ParameterValue: aws.String("backend"),
				},
			},

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
					ParameterValue: aws.String("frontend,backend"),
				},
				{
					ParameterKey:   aws.String(envParamInternalALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamEFSWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamNATWorkloadsKey),
					ParameterValue: aws.String("backend"),
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
		"should not include old parameters that are deleted": {
			input: deploymentInput,
			oldParams: []*cloudformation.Parameter{
				{
					ParameterKey: aws.String("deprecated"),
				},
			},

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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			env := &EnvStackConfig{
				in:         tc.input,
				prevParams: tc.oldParams,
			}
			params, err := env.Parameters()
			require.NoError(t, err)
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

func TestBootstrapEnv_Template(t *testing.T) {
	testCases := map[string]struct {
		in             *deploy.CreateEnvironmentInput
		setupMock      func(m *mocks.MockenvReadParser)
		expectedOutput string
		wantedError    error
	}{
		"error parsing the template": {
			in: &deploy.CreateEnvironmentInput{},
			setupMock: func(m *mocks.MockenvReadParser) {
				m.EXPECT().ParseEnvBootstrap(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"should return template body when present": {
			in: &deploy.CreateEnvironmentInput{
				ArtifactBucketARN:    "mockBucketARN",
				ArtifactBucketKeyARN: "mockBucketKeyARN",
			},
			setupMock: func(m *mocks.MockenvReadParser) {
				m.EXPECT().ParseEnvBootstrap(gomock.Any(), gomock.Any()).DoAndReturn(func(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error) {
					require.Equal(t, &template.EnvOpts{
						ArtifactBucketARN:    "mockBucketARN",
						ArtifactBucketKeyARN: "mockBucketKeyARN",
					}, data)
					return &template.Content{Buffer: bytes.NewBufferString("mockTemplate")}, nil
				})
			},
			expectedOutput: "mockTemplate",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockParser := mocks.NewMockenvReadParser(ctrl)
			tc.setupMock(mockParser)
			bootstrapStack := &BootstrapEnvStackConfig{
				in:     tc.in,
				parser: mockParser,
			}

			// WHEN
			got, err := bootstrapStack.Template()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedOutput, got)
			}
		})
	}
}

func TestBootstrapEnv_Parameters(t *testing.T) {
	testCases := map[string]struct {
		input *deploy.CreateEnvironmentInput
		want  []*cloudformation.Parameter
	}{
		"returns correct parameters": {
			input: &deploy.CreateEnvironmentInput{
				App: deploy.AppInformation{
					Name:                "mockApp",
					AccountPrincipalARN: "mockAccountPrincipalARN",
				},
				Name: "mockEnv",
			},
			want: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(envParamAppNameKey),
					ParameterValue: aws.String("mockApp"),
				},
				{
					ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
					ParameterValue: aws.String("mockAccountPrincipalARN"),
				},
				{
					ParameterKey:   aws.String(envParamEnvNameKey),
					ParameterValue: aws.String("mockEnv"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			bootstrap := &BootstrapEnvStackConfig{
				in: tc.input,
			}
			params, err := bootstrap.Parameters()
			require.NoError(t, err)
			require.ElementsMatch(t, tc.want, params)
		})
	}
}

func TestBootstrapEnv_Tags(t *testing.T) {
	bootstrap := &BootstrapEnvStackConfig{
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
	require.ElementsMatch(t, expectedTags, bootstrap.Tags())
}

func TestBootstrapEnv_StackName(t *testing.T) {
	bootstrap := &BootstrapEnvStackConfig{
		in: &deploy.CreateEnvironmentInput{
			App: deploy.AppInformation{
				Name: "mockApp",
			},
			Name: "mockEnv",
		},
	}
	require.Equal(t, "mockApp-mockEnv", bootstrap.StackName())
}

func TestBootstrapEnv_ToEnv(t *testing.T) {
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
			envStack := &BootstrapEnvStackConfig{
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
			"CertificateValidationFunction": "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey1",
			"DNSDelegationFunction":         "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey2",
			"CustomDomainFunction":          "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey4",
		},
		Mft: &manifest.Environment{
			Workload: manifest.Workload{
				Name: aws.String("env"),
				Type: aws.String("Environment"),
			},
		},
		RawMft: []byte(`name: env
type: Environment
`),
	}
}
