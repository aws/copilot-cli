// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/template/templatetest"

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
	t.Cleanup(func() {
		fs = realEmbedFS
	})

	t.Run("error parsing template", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// GIVEN
		inEnvConfig := mockDeployEnvironmentInput()
		mockParser := mocks.NewMockembedFS(ctrl)
		fs = mockParser

		// EXPECT
		mockParser.EXPECT().Read(gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("data")}, nil).AnyTimes()
		mockParser.EXPECT().ParseEnv(gomock.Any()).Return(nil, errors.New("some error"))

		// WHEN
		envStack, err := NewEnvConfigFromExistingStack(inEnvConfig, "mockPreviousForceUpdateID", nil)
		require.NoError(t, err)
		_, err = envStack.Template()

		// THEN
		require.EqualError(t, errors.New("some error"), err.Error())
	})
	t.Run("error parsing addons extra parameters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// GIVEN
		inEnvConfig := mockDeployEnvironmentInput()
		mockAddonsConfig := mocks.NewMockNestedStackConfigurer(ctrl)
		inEnvConfig.Addons = &Addons{
			S3ObjectURL: "mockAddonsURL",
			Stack:       mockAddonsConfig,
		}
		fs = templatetest.Stub{}

		// EXPECT
		mockAddonsConfig.EXPECT().Parameters().Return("", errors.New("some error"))

		// WHEN
		envStack, err := NewEnvConfigFromExistingStack(inEnvConfig, "mockPreviousForceUpdateID", nil)
		require.NoError(t, err)
		_, err = envStack.Template()

		// THEN
		require.EqualError(t, errors.New("parse extra parameters for environment addons: some error"), err.Error())
	})
	t.Run("should contain addons information when addons are present", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// GIVEN
		inEnvConfig := mockDeployEnvironmentInput()
		mockAddonsConfig := mocks.NewMockNestedStackConfigurer(ctrl)
		mockAddonsConfig.EXPECT().Parameters().Return("mockAddonsExtraParameters", nil)
		inEnvConfig.Addons = &Addons{
			S3ObjectURL: "mockAddonsURL",
			Stack:       mockAddonsConfig,
		}
		mockParser := mocks.NewMockembedFS(ctrl)
		mockParser.EXPECT().Read(gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("data")}, nil).AnyTimes()
		mockParser.EXPECT().ParseEnv(gomock.Any()).DoAndReturn(func(data *template.EnvOpts) (*template.Content, error) {
			require.Equal(t, &template.Addons{
				URL:         "mockAddonsURL",
				ExtraParams: "mockAddonsExtraParameters",
			}, data.Addons)
			return &template.Content{Buffer: bytes.NewBufferString("mockTemplate")}, nil
		})
		fs = mockParser

		// WHEN
		envStack, err := NewEnvConfigFromExistingStack(inEnvConfig, "mockPreviousForceUpdateID", nil)
		require.NoError(t, err)
		got, err := envStack.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, mockTemplate, got)
	})
	t.Run("should use new force update ID when asked", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// GIVEN
		inEnvConfig := mockDeployEnvironmentInput()
		inEnvConfig.ForceUpdate = true
		mockParser := mocks.NewMockembedFS(ctrl)
		mockParser.EXPECT().Read(gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("data")}, nil).AnyTimes()
		mockParser.EXPECT().ParseEnv(gomock.Any()).DoAndReturn(func(data *template.EnvOpts) (*template.Content, error) {
			require.NotEqual(t, "mockPreviousForceUpdateID", data.ForceUpdateID)
			return &template.Content{Buffer: bytes.NewBufferString("mockTemplate")}, nil
		})
		fs = mockParser

		// WHEN
		envStack, err := NewEnvConfigFromExistingStack(inEnvConfig, "mockPreviousForceUpdateID", nil)
		require.NoError(t, err)
		got, err := envStack.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, mockTemplate, got)
	})
	t.Run("should return template body when present", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// GIVEN
		inEnvConfig := mockDeployEnvironmentInput()
		mockParser := mocks.NewMockembedFS(ctrl)
		mockParser.EXPECT().Read(gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("data")}, nil).AnyTimes()
		mockParser.EXPECT().ParseEnv(gomock.Any()).DoAndReturn(func(data *template.EnvOpts) (*template.Content, error) {
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
					FlowLogs:            nil,
				},
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
				ArtifactBucketARN:  "arn:aws:s3:::mockbucket",
				SerializedManifest: "name: env\ntype: Environment\n",
				ForceUpdateID:      "mockPreviousForceUpdateID",
			}, data)
			return &template.Content{Buffer: bytes.NewBufferString("mockTemplate")}, nil
		})
		fs = mockParser

		// WHEN
		envStack, err := NewEnvConfigFromExistingStack(inEnvConfig, "mockPreviousForceUpdateID", nil)
		require.NoError(t, err)
		got, err := envStack.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, mockTemplate, got)
	})
	t.Run("should return template body with local custom resources when not uploaded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// GIVEN
		inEnvConfig := mockDeployEnvironmentInput()
		inEnvConfig.CustomResourcesURLs = nil
		mockParser := mocks.NewMockembedFS(ctrl)
		mockParser.EXPECT().Read(gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("data")}, nil).AnyTimes()
		mockParser.EXPECT().ParseEnv(gomock.Any()).DoAndReturn(func(data *template.EnvOpts) (*template.Content, error) {
			require.Equal(t, map[string]template.S3ObjectLocation{
				"CertificateReplicatorFunction": {
					Bucket: "mockbucket",
					Key:    "manual/scripts/custom-resources/certificatereplicatorfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"CertificateValidationFunction": {
					Bucket: "mockbucket",
					Key:    "manual/scripts/custom-resources/certificatevalidationfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"DNSDelegationFunction": {
					Bucket: "mockbucket",
					Key:    "manual/scripts/custom-resources/dnsdelegationfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"UniqueJSONValuesFunction": {
					Bucket: "mockbucket",
					Key:    "manual/scripts/custom-resources/uniquejsonvaluesfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"CustomDomainFunction": {
					Bucket: "mockbucket",
					Key:    "manual/scripts/custom-resources/customdomainfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"BucketCleanerFunction": {
					Bucket: "mockbucket",
					Key:    "manual/scripts/custom-resources/bucketcleanerfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
			}, data.CustomResources)
			return &template.Content{Buffer: bytes.NewBufferString("mockTemplate")}, nil
		})
		fs = mockParser

		// WHEN
		envStack, err := NewEnvConfigFromExistingStack(inEnvConfig, "mockPreviousForceUpdateID", nil)
		require.NoError(t, err)
		got, err := envStack.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, mockTemplate, got)
	})
}

func TestEnv_Parameters(t *testing.T) {
	t.Cleanup(func() {
		fs = realEmbedFS
	})
	fs = templatetest.Stub{}

	deploymentInput := mockDeployEnvironmentInput()
	deploymentInputWithDNS := mockDeployEnvironmentInput()
	deploymentInputWithDNS.App.Domain = "ecs.aws"
	deploymentInputWithPrivateDNS := mockDeployEnvironmentInput()
	deploymentInputWithPrivateDNS.Mft.HTTPConfig.Private.Certificates = []string{"arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"}
	testCases := map[string]struct {
		input     *EnvConfig
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
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String(""),
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
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String(""),
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
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String(""),
				},
			},
		},
		"should use default value for new EnvControllerParameters": {
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
				{
					ParameterKey:   aws.String(EnvParamAliasesKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamEFSWorkloadsKey),
					ParameterValue: aws.String(""),
				},
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
					ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
					ParameterValue: aws.String("mockenv.mockapp.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String("rdws-backend"),
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
					ParameterValue: aws.String("mockenv.mockapp.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String("rdws-backend"),
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
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String("rdws-backend"),
				},
				{
					ParameterKey:   aws.String(EnvParamAliasesKey),
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
					ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
					ParameterValue: aws.String("mockenv.mockapp.local"),
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
					ParameterValue: aws.String("mockenv.mockapp.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String("rdws-backend"),
				},
			},
		},
		"should not include old parameters that are deleted": {
			input: deploymentInput,
			oldParams: []*cloudformation.Parameter{
				{
					ParameterKey: aws.String("deprecated"),
				},
				{
					ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamNATWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamAliasesKey),
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
					ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
					ParameterValue: aws.String("mockenv.mockapp.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String(""),
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
					ParameterValue: aws.String("mockenv.mockapp.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String(""),
				},
			},
		},
		"should reuse old service discovery endpoint value": {
			input: deploymentInput,
			oldParams: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
					ParameterValue: aws.String("app.local"),
				},
				{
					ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(envParamNATWorkloadsKey),
					ParameterValue: aws.String(""),
				},
				{
					ParameterKey:   aws.String(EnvParamAliasesKey),
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
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String(""),
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
					ParameterValue: aws.String("app.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String(""),
				},
			},
		},
		"should use app.local endpoint service discovery endpoint if it is a new parameter": {
			input: deploymentInput,
			oldParams: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
					ParameterValue: aws.String(""),
				},
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
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String(""),
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
					ParameterValue: aws.String("project.local"),
				},
				{
					ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
					ParameterValue: aws.String(""),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			env, err := NewEnvConfigFromExistingStack(tc.input, "", tc.oldParams)
			require.NoError(t, err)
			params, err := env.Parameters()
			require.NoError(t, err)
			require.ElementsMatch(t, tc.want, params)
		})
	}
}

func TestEnv_Tags(t *testing.T) {
	env := &Env{
		in: &EnvConfig{
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
	env := &Env{
		in: deploymentInput,
	}
	require.Equal(t, fmt.Sprintf("%s-%s", deploymentInput.App.Name, deploymentInput.Name), env.StackName())
}

func TestBootstrapEnv_Template(t *testing.T) {
	testCases := map[string]struct {
		in             *EnvConfig
		setupMock      func(m *mocks.MockenvReadParser)
		expectedOutput string
		wantedError    error
	}{
		"error parsing the template": {
			in: &EnvConfig{},
			setupMock: func(m *mocks.MockenvReadParser) {
				m.EXPECT().ParseEnvBootstrap(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"should return template body when present": {
			in: &EnvConfig{
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
			bootstrapStack := &BootstrapEnv{
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
		input *EnvConfig
		want  []*cloudformation.Parameter
	}{
		"returns correct parameters": {
			input: &EnvConfig{
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
			bootstrap := &BootstrapEnv{
				in: tc.input,
			}
			params, err := bootstrap.Parameters()
			require.NoError(t, err)
			require.ElementsMatch(t, tc.want, params)
		})
	}
}

func TestBootstrapEnv_Tags(t *testing.T) {
	bootstrap := &BootstrapEnv{
		in: &EnvConfig{
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
	bootstrap := &BootstrapEnv{
		in: &EnvConfig{
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
			envStack := &BootstrapEnv{
				in: mockDeployInput,
			}
			got, err := envStack.ToEnvMetadata(tc.mockStack)

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

func mockDeployEnvironmentInput() *EnvConfig {
	return &EnvConfig{
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
		ArtifactBucketARN: "arn:aws:s3:::mockbucket",
		Mft: &manifest.Environment{
			Workload: manifest.Workload{
				Name: aws.String("env"),
				Type: aws.String("Environment"),
			},
		},
		RawMft: `name: env
type: Environment
`,
	}
}
