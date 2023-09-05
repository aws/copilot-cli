// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/copilot-cli/internal/pkg/term/selector"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
)

type initEnvMocks struct {
	sessProvider *mocks.MocksessionProvider
	prompt       *mocks.Mockprompter
	selVPC       *mocks.Mockec2Selector
	selCreds     *mocks.MockcredsSelector
	ec2Client    *mocks.Mockec2Client
	selApp       *mocks.MockappSelector
	store        *mocks.Mockstore
	envLister    *mocks.MockwsEnvironmentsLister
	wsAppName    string
}

func TestInitEnvOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inEnvName string
		inAppName string
		inDefault bool

		inVPCID              string
		inPublicIDs          []string
		inPrivateIDs         []string
		inInternalALBSubnets []string

		inVPCCIDR     net.IPNet
		inAZs         []string
		inPublicCIDRs []string

		inProfileName     string
		inAccessKeyID     string
		inSecretAccessKey string
		inSessionToken    string

		setupMocks func(m *initEnvMocks)

		wantedErrMsg string
	}{
		"valid environment creation": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test-pdx").Return(nil, &config.ErrNoSuchEnvironment{})
			},
		},
		"fail if command not run under a workspace": {
			wantedErrMsg: "could not find an application attached to this workspace, please run `app init` first",
		},
		"fail if using different app name from the workspace": {
			inAppName: "demo",
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
			},
			wantedErrMsg: "cannot specify app demo because the workspace is already registered with app phonetool",
		},
		"fail if cannot validate application": {
			inAppName: "phonetool",

			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))
			},
			wantedErrMsg: "get application phonetool configuration: some error",
		},
		"invalid environment name": {
			inEnvName: "123env",
			inAppName: "phonetool",

			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
			wantedErrMsg: fmt.Sprintf("environment name 123env is invalid: %s", errBasicNameRegexNotMatched),
		},
		"should error if environment already exists": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",

			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test-pdx").Return(nil, nil)
				m.envLister.EXPECT().ListEnvironments().Return([]string{}, nil)
			},
			wantedErrMsg: "environment test-pdx already exists",
		},
		"should skip error if environment already exists in current workspace": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",

			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test-pdx").Return(nil, nil)
				m.envLister.EXPECT().ListEnvironments().Return([]string{"test-pdx"}, nil)
			},
		},
		"cannot specify both vpc resources importing flags and configuring flags": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",

			inPublicCIDRs: []string{"mockCIDR"},
			inPublicIDs:   []string{"mockID", "anotherMockID"},
			inVPCCIDR: net.IPNet{
				IP:   net.IP{10, 1, 232, 0},
				Mask: net.IPMask{255, 255, 255, 0},
			},
			inVPCID: "mockID",

			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test-pdx").Return(nil, &config.ErrNoSuchEnvironment{})
			},

			wantedErrMsg: "cannot specify both import vpc flags and configure vpc flags",
		},
		"cannot import or configure resources if use default flag is set": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",

			inDefault: true,
			inVPCID:   "mockID",
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test-pdx").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			wantedErrMsg: fmt.Sprintf("cannot import or configure vpc if --%s is set", defaultConfigFlag),
		},
		"should err if both profile and access key id are set": {
			inAppName:     "phonetool",
			inEnvName:     "test",
			inProfileName: "default",
			inAccessKeyID: "AKIAIOSFODNN7EXAMPLE",
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			wantedErrMsg: "cannot specify both --profile and --aws-access-key-id",
		},
		"should err if both profile and secret access key are set": {
			inAppName:         "phonetool",
			inEnvName:         "test",
			inProfileName:     "default",
			inSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			wantedErrMsg: "cannot specify both --profile and --aws-secret-access-key",
		},
		"should err if both profile and session token are set": {
			inAppName:      "phonetool",
			inEnvName:      "test",
			inProfileName:  "default",
			inSessionToken: "verylongtoken",
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			wantedErrMsg: "cannot specify both --profile and --aws-session-token",
		},
		"should err if fewer than two private subnets are set:": {
			inVPCID:      "mockID",
			inPublicIDs:  []string{"mockID", "anotherMockID"},
			inPrivateIDs: []string{"mockID"},
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
			wantedErrMsg: "at least two private subnets must be imported",
		},
		"should err if fewer than two availability zones are provided": {
			inAZs: []string{"us-east-1a"},
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
			wantedErrMsg: "at least two availability zones must be provided to enable Load Balancing",
		},
		"invalid VPC resource import (no VPC, 1 public, 2 private)": {
			inPublicIDs:  []string{"mockID"},
			inPrivateIDs: []string{"mockID", "anotherMockID"},
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
			wantedErrMsg: "at least two public subnets must be imported to enable Load Balancing",
		},
		"valid VPC resource import (0 public, 3 private)": {
			inVPCID:      "mockID",
			inPublicIDs:  []string{},
			inPrivateIDs: []string{"mockID", "anotherMockID", "yetAnotherMockID"},
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
		},
		"valid VPC resource import (3 public, 2 private)": {
			inVPCID:      "mockID",
			inPublicIDs:  []string{"mockID", "anotherMockID", "yetAnotherMockID"},
			inPrivateIDs: []string{"mockID", "anotherMockID"},
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
		},
		"cannot specify internal ALB subnet placement with default config": {
			inDefault:            true,
			inInternalALBSubnets: []string{"mockSubnet", "anotherMockSubnet"},
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
			wantedErrMsg: "subnets 'mockSubnet, anotherMockSubnet' specified for internal ALB placement, but those subnets are not imported",
		},
		"cannot specify internal ALB subnet placement with adjusted VPC resources": {
			inPublicCIDRs:        []string{"mockCIDR"},
			inInternalALBSubnets: []string{"mockSubnet", "anotherMockSubnet"},
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
			wantedErrMsg: "subnets 'mockSubnet, anotherMockSubnet' specified for internal ALB placement, but those subnets are not imported",
		},
		"invalid specification of internal ALB subnet placement": {
			inPrivateIDs:         []string{"mockID", "mockSubnet", "anotherMockSubnet"},
			inInternalALBSubnets: []string{"mockSubnet", "notMockSubnet"},
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
			wantedErrMsg: "subnets 'mockSubnet, notMockSubnet' were designated for ALB placement, but they were not all imported",
		},
		"valid specification of internal ALB subnet placement": {
			inPrivateIDs:         []string{"mockID", "mockSubnet", "anotherMockSubnet"},
			inInternalALBSubnets: []string{"mockSubnet", "anotherMockSubnet"},
			setupMocks: func(m *initEnvMocks) {
				m.wsAppName = "phonetool"
				m.store.EXPECT().GetApplication("phonetool").Return(nil, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &initEnvMocks{
				store:     mocks.NewMockstore(ctrl),
				envLister: mocks.NewMockwsEnvironmentsLister(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}

			// GIVEN
			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					name:               tc.inEnvName,
					defaultConfig:      tc.inDefault,
					internalALBSubnets: tc.inInternalALBSubnets,
					adjustVPC: adjustVPCVars{
						AZs:               tc.inAZs,
						PublicSubnetCIDRs: tc.inPublicCIDRs,
						CIDR:              tc.inVPCCIDR,
					},
					importVPC: importVPCVars{
						PublicSubnetIDs:  tc.inPublicIDs,
						PrivateSubnetIDs: tc.inPrivateIDs,
						ID:               tc.inVPCID,
					},
					appName: tc.inAppName,
					profile: tc.inProfileName,
					tempCreds: tempCredsVars{
						AccessKeyID:     tc.inAccessKeyID,
						SecretAccessKey: tc.inSecretAccessKey,
						SessionToken:    tc.inSessionToken,
					},
				},
				store:     m.store,
				wsAppName: m.wsAppName,
				envLister: m.envLister,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedErrMsg != "" {
				require.EqualError(t, err, tc.wantedErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInitEnvOpts_Ask(t *testing.T) {
	const (
		mockApp         = "test-app"
		mockEnv         = "test"
		mockProfile     = "default"
		mockVPCCIDR     = "10.10.10.10/24"
		mockSubnetCIDRs = "10.10.10.10/24,10.10.10.10/24"
		mockRegion      = "us-west-2"
	)
	mockErr := errors.New("some error")
	mockSession := &session.Session{
		Config: &aws.Config{
			Region: aws.String(mockRegion),
		},
	}
	mockPublicSubnetInput := selector.SubnetsInput{
		Msg:      envInitPublicSubnetsSelectPrompt,
		Help:     "",
		VPCID:    "mockVPC",
		IsPublic: true,
	}
	mockPrivateSubnetInput := selector.SubnetsInput{
		Msg:      envInitPrivateSubnetsSelectPrompt,
		Help:     "",
		VPCID:    "mockVPC",
		IsPublic: false,
	}

	testCases := map[string]struct {
		inAppName            string
		inEnv                string
		inProfile            string
		inTempCreds          tempCredsVars
		inRegion             string
		inDefault            bool
		inImportVPCVars      importVPCVars
		inAdjustVPCVars      adjustVPCVars
		inInternalALBSubnets []string

		getMockCredsSelector func() (credsSelector, error)
		setupMocks           func(mocks initEnvMocks)

		wantedError error
	}{
		"fail to get env name": {
			inAppName: mockApp,
			setupMocks: func(m initEnvMocks) {
				gomock.InOrder(
					m.prompt.EXPECT().
						Get(envInitNamePrompt, envInitNameHelpPrompt, gomock.Any(), gomock.Any()).
						Return("", mockErr),
				)
			},
			wantedError: fmt.Errorf("get environment name: some error"),
		},
		"should error if environment already exists": {
			inAppName: mockApp,
			setupMocks: func(m initEnvMocks) {
				gomock.InOrder(
					m.prompt.EXPECT().
						Get(envInitNamePrompt, envInitNameHelpPrompt, gomock.Any(), gomock.Any()).
						Return("test", nil),
					m.store.EXPECT().GetEnvironment(mockApp, mockEnv).Return(nil, nil),
					m.envLister.EXPECT().ListEnvironments().Return([]string{}, nil),
				)
			},
			wantedError: errors.New("environment test already exists"),
		},
		"should skip error if environment already exists in workspace": {
			inAppName: mockApp,
			inProfile: mockProfile,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				gomock.InOrder(
					m.prompt.EXPECT().
						Get(envInitNamePrompt, envInitNameHelpPrompt, gomock.Any(), gomock.Any()).
						Return("test", nil),
					m.store.EXPECT().GetEnvironment(mockApp, mockEnv).Return(nil, nil),
					m.envLister.EXPECT().ListEnvironments().Return([]string{mockEnv}, nil),
					m.sessProvider.EXPECT().FromProfile(mockProfile).Return(&session.Session{
						Config: &aws.Config{
							Region: aws.String("us-west-2"),
						},
					}, nil).AnyTimes(),
				)
			},
		},
		"should create a session from a named profile if flag is provided": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(mockProfile).Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
			},
		},
		"should create a session from temporary creds if flags are provided": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inTempCreds: tempCredsVars{
				AccessKeyID:     "abcd",
				SecretAccessKey: "efgh",
			},
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromStaticCreds("abcd", "efgh", "").Return(mockSession, nil)
			},
		},
		"should prompt for credentials if no profile or temp creds flags are provided": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.selCreds.EXPECT().Creds("Which credentials would you like to use to create test?", gomock.Any()).Return(mockSession, nil)
			},
		},
		"should fallback on default session if credentials cant be found": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inDefault: true,
			getMockCredsSelector: func() (credsSelector, error) {
				return nil, mockErr
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
			},
		},
		"should prompt for region if user configuration does not have one": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(&session.Session{
					Config: &aws.Config{},
				}, nil)
				m.prompt.EXPECT().Get("Which region?", gomock.Any(), nil, gomock.Any(), gomock.Any()).Return("us-west-2", nil)
			},
		},
		"should skip prompting for region if flag is provided": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inRegion:  mockRegion,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(&session.Session{
					Config: &aws.Config{},
				}, nil)
				m.prompt.EXPECT().Get("Which region?", gomock.Any(), nil, gomock.Any(), gomock.Any()).Times(0)
			},
		},
		"should not prompt for configuring environment if default config flag is true": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
		},
		"fail to select whether to adjust or import resources": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return("", mockErr)
			},
			wantedError: fmt.Errorf("select adjusting or importing resources: some error"),
		},
		"success with no custom resources": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitDefaultConfigSelectOption, nil)
			},
		},
		"fail to select VPC": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("", mockErr)
			},
			wantedError: fmt.Errorf("select VPC: some error"),
		},
		"fail to check if VPC has DNS support": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(false, mockErr)
			},
			wantedError: fmt.Errorf("check if VPC mockVPC has DNS support enabled: some error"),
		},
		"fail to import VPC has no DNS support": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(false, nil)
			},
			wantedError: fmt.Errorf("VPC mockVPC has no DNS support enabled"),
		},
		"fail to select public subnets": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return(nil, mockErr)
			},
			wantedError: fmt.Errorf("select public subnets: some error"),
		},
		"fail to select TWO public subnets": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{"mockPublicSubnet"}, nil)
			},
			wantedError: fmt.Errorf("select public subnets: at least two public subnets must be selected to enable Load Balancing"),
		},
		"fail to select private subnets": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{"mockPublicSubnet", "anotherMockPublicSubnet"}, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return(nil, mockErr)
			},
			wantedError: fmt.Errorf("select private subnets: some error"),
		},
		"fail to select TWO private subnets": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{"mockPublicSubnet", "anotherMockPublicSubnet"}, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return([]string{"mockPrivateSubnet"}, nil)
			},
			wantedError: fmt.Errorf("select private subnets: at least two private subnets must be selected"),
		},
		"success with selecting zero public subnets and two private subnets": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{}, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return([]string{"mockPrivateSubnet", "anotherMockPrivateSubnet"}, nil)
			},
		},
		"success with selecting two public subnets and zero private subnets": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{"mockPublicSubnet", "anotherMockPublicSubnet"}, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return([]string{}, nil)
			},
		},
		"success with importing env resources with no flags": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{"mockPublicSubnet", "anotherMockPublicSubnet"}, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return([]string{"mockPrivateSubnet", "anotherMockPrivateSubnet"}, nil)
			},
		},
		"success with importing env resources with flags": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inImportVPCVars: importVPCVars{
				ID:               "mockVPCID",
				PrivateSubnetIDs: []string{"mockPrivateSubnetID", "anotherMockPrivateSubnetID"},
				PublicSubnetIDs:  []string{"mockPublicSubnetID", "anotherMockPublicSubnetID"},
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPCID").Return(true, nil)
			},
		},
		"prompt for subnets if only VPC passed with flag": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inImportVPCVars: importVPCVars{
				ID: "mockVPC",
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{"mockPublicSubnet", "anotherMockPublicSubnet"}, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return([]string{"mockPrivateSubnet", "anotherMockPrivateSubnet"}, nil)
			},
		},
		"prompt for VPC if only subnets passed with flags": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inImportVPCVars: importVPCVars{
				PrivateSubnetIDs: []string{"mockPrivateSubnetID", "anotherMockPrivateSubnetID"},
				PublicSubnetIDs:  []string{"mockPublicSubnetID", "anotherMockPublicSubnetID"},
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
			},
		},
		"prompt for public subnets if only private subnets and VPC passed with flags": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inImportVPCVars: importVPCVars{
				ID:               "mockVPC",
				PrivateSubnetIDs: []string{"mockPrivateSubnetID", "anotherMockPrivateSubnetID"},
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{"mockPublicSubnet", "anotherMockPublicSubnet"}, nil)
			},
		},
		"prompt for private subnets if only public subnets and VPC passed with flags": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inImportVPCVars: importVPCVars{
				ID:              "mockVPC",
				PublicSubnetIDs: []string{"mockPublicSubnetID", "anotherMockPublicSubnetID"},
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return([]string{"mockPrivateSubnet", "anotherMockPrivateSubnet"}, nil)
			},
		},
		"after prompting for missing imported VPC resources, validate if internalALBSubnets set": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inImportVPCVars: importVPCVars{
				ID: "mockVPC",
			},
			inInternalALBSubnets: []string{"nonexistentSubnet", "anotherNonexistentSubnet"},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{"mockPublicSubnet", "anotherMockPublicSubnet"}, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return([]string{"mockPrivateSubnet", "anotherMockPrivateSubnet"}, nil)
			},

			wantedError: errors.New("subnets 'nonexistentSubnet, anotherNonexistentSubnet' were designated for ALB placement, but they were not all imported"),
		},
		"error if no subnets selected": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inImportVPCVars: importVPCVars{
				ID: "mockVPC",
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return(nil, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return(nil, nil)
			},
			wantedError: fmt.Errorf("VPC must have subnets in order to proceed with environment creation"),
		},
		"only prompt for imported resources if internalALBSubnets set": {
			inAppName:            mockApp,
			inEnv:                mockEnv,
			inProfile:            mockProfile,
			inInternalALBSubnets: []string{"mockPrivateSubnet", "anotherMockPrivateSubnet"},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().Subnets(mockPublicSubnetInput).
					Return([]string{"mockPublicSubnet", "anotherMockPublicSubnet"}, nil)
				m.selVPC.EXPECT().Subnets(mockPrivateSubnetInput).
					Return([]string{"mockPrivateSubnet", "anotherMockPrivateSubnet"}, nil)
			},
		},
		"fail to get VPC CIDR": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, envInitVPCCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return("", mockErr)
			},
			wantedError: fmt.Errorf("get VPC CIDR: some error"),
		},
		"should return err when failed to retrieve list of AZs to adjust": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockVPCCIDR, nil)
				m.ec2Client.EXPECT().ListAZs().Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("list availability zones for region %s: some error", mockRegion),
		},
		"should return err if the number of available AZs does not meet the minimum": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockVPCCIDR, nil)
				m.ec2Client.EXPECT().ListAZs().Return([]ec2.AZ{
					{
						Name: "us-east-1a",
					},
				}, nil)
			},
			wantedError: fmt.Errorf("requires at least 2 availability zones (us-east-1a) in region %s", mockRegion),
		},
		"fail to get public subnet CIDRs": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockVPCCIDR, nil)
				m.ec2Client.EXPECT().ListAZs().Return([]ec2.AZ{
					{
						Name: "us-east-1a",
					},
					{
						Name: "us-east-1b",
					},
				}, nil)
				m.prompt.EXPECT().MultiSelect(envInitAdjustAZPrompt, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]string{"us-east-1a", "us-east-1b"}, nil)
				m.prompt.EXPECT().Get(envInitPublicCIDRPrompt, envInitPublicCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return("", mockErr)
			},
			wantedError: fmt.Errorf("get public subnet CIDRs: some error"),
		},
		"fail to get private subnet CIDRs": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockVPCCIDR, nil)
				m.ec2Client.EXPECT().ListAZs().Return([]ec2.AZ{
					{
						Name: "us-east-1a",
					},
					{
						Name: "us-east-1b",
					},
				}, nil)
				m.prompt.EXPECT().MultiSelect(envInitAdjustAZPrompt, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]string{"us-east-1a", "us-east-1b"}, nil)
				m.prompt.EXPECT().Get(envInitPublicCIDRPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(mockSubnetCIDRs, nil)
				m.prompt.EXPECT().Get(envInitPrivateCIDRPrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", mockErr)
			},
			wantedError: fmt.Errorf("get private subnet CIDRs: some error"),
		},
		"success with adjusting default env config with no flags": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes, gomock.Any()).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, envInitVPCCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return(mockVPCCIDR, nil)
				m.ec2Client.EXPECT().ListAZs().Return([]ec2.AZ{
					{
						Name: "us-east-1a",
					},
					{
						Name: "us-east-1b",
					},
				}, nil)
				m.prompt.EXPECT().MultiSelect(envInitAdjustAZPrompt, gomock.Any(), []string{"us-east-1a", "us-east-1b"}, gomock.Any(), gomock.Any()).
					Return([]string{"us-east-1a", "us-east-1b"}, nil)
				m.prompt.EXPECT().Get(envInitPublicCIDRPrompt, envInitPublicCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return(mockSubnetCIDRs, nil)
				m.prompt.EXPECT().Get(envInitPrivateCIDRPrompt, envInitPrivateCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return(mockSubnetCIDRs, nil)
			},
		},
		"success with adjusting default env config with flags": {
			inAppName: mockApp,
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inAdjustVPCVars: adjustVPCVars{
				CIDR: net.IPNet{
					IP:   net.IP{10, 1, 232, 0},
					Mask: net.IPMask{255, 255, 255, 0},
				},
				AZs:                []string{"us-east-1a", "us-east-1b"},
				PrivateSubnetCIDRs: []string{"mockPrivateCIDR1", "mockPrivateCIDR2"},
				PublicSubnetCIDRs:  []string{"mockPublicCIDR1", "mockPublicCIDR2"},
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := initEnvMocks{
				sessProvider: mocks.NewMocksessionProvider(ctrl),
				prompt:       mocks.NewMockprompter(ctrl),
				selVPC:       mocks.NewMockec2Selector(ctrl),
				selCreds:     mocks.NewMockcredsSelector(ctrl),
				ec2Client:    mocks.NewMockec2Client(ctrl),
				selApp:       mocks.NewMockappSelector(ctrl),
				store:        mocks.NewMockstore(ctrl),
				envLister:    mocks.NewMockwsEnvironmentsLister(ctrl),
			}

			tc.setupMocks(mocks)
			// GIVEN
			addEnv := &initEnvOpts{
				initEnvVars: initEnvVars{
					appName:            tc.inAppName,
					name:               tc.inEnv,
					profile:            tc.inProfile,
					tempCreds:          tc.inTempCreds,
					region:             tc.inRegion,
					defaultConfig:      tc.inDefault,
					adjustVPC:          tc.inAdjustVPCVars,
					importVPC:          tc.inImportVPCVars,
					internalALBSubnets: tc.inInternalALBSubnets,
				},
				sessProvider: mocks.sessProvider,
				selVPC:       mocks.selVPC,
				selCreds: func() (credsSelector, error) {
					if tc.getMockCredsSelector != nil {
						return tc.getMockCredsSelector()
					}
					return mocks.selCreds, nil
				},
				ec2Client: mocks.ec2Client,
				prompt:    mocks.prompt,
				selApp:    mocks.selApp,
				store:     mocks.store,
				envLister: mocks.envLister,
			}

			// WHEN
			err := addEnv.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, mockEnv, addEnv.name, "expected environment names to match")
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

type initEnvExecuteMocks struct {
	store            *mocks.Mockstore
	deployer         *mocks.Mockdeployer
	identity         *mocks.MockidentityService
	progress         *mocks.Mockprogress
	iam              *mocks.MockroleManager
	cfn              *mocks.MockstackExistChecker
	appCFN           *mocks.MockappResourcesGetter
	manifestWriter   *mocks.MockenvironmentManifestWriter
	appVersionGetter *mocks.MockversionGetter
}

func TestInitEnvOpts_Execute(t *testing.T) {
	const (
		mockAppVersion       = "v0.0.0"
		mockCurrVersion      = "v1.29.0"
		mockFutureAppVersion = "v2.0.0"
	)
	mockError := errors.New("some error")
	testCases := map[string]struct {
		enableContainerInsights bool
		allowDowngrade          bool
		setupMocks              func(m *initEnvExecuteMocks)
		wantedErrorS            string
	}{
		"returns error when failed to get app version": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return("", mockError)
			},
			wantedErrorS: "get template version of application phonetool: some error",
		},
		"returns error when cannot downgrade app version": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockFutureAppVersion, nil)
			},
			wantedErrorS: "cannot downgrade application \"phonetool\" (currently in version v2.0.0) to version v1.29.0",
		},
		"returns app exists error": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().GetApplication("phonetool").Return(nil, mockError)
			},
			wantedErrorS: "some error",
		},
		"returns identity get error": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.identity.EXPECT().Get().Return(identity.Caller{}, errors.New("some identity error"))
			},
			wantedErrorS: "get identity: some identity error",
		},
		"fail to write manifest": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().CreateEnvironment(gomock.Any()).Times(0)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("", mockError)
			},
			wantedErrorS: "write environment manifest: some error",
		},
		"failed to create stack set instance": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().CreateEnvironment(gomock.Any()).Times(0)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("/environments/test/manifest.yml", nil)
				m.iam.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				m.deployer.EXPECT().AddEnvToApp(&deploycfn.AddEnvToAppOpts{
					App:          &config.Application{Name: "phonetool"},
					EnvName:      "test",
					EnvAccountID: "1234",
					EnvRegion:    "us-west-2",
				}).Return(errors.New("some cfn error"))
			},
			wantedErrorS: "add env test to application phonetool: some cfn error",
		},
		"errors cannot get app resources by region": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("/environments/test/manifest.yml", nil)
				m.iam.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				m.deployer.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
				m.appCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(nil, mockError)
			},
			wantedErrorS: "get app resources: some error",
		},
		"deletes retained IAM roles if environment stack fails creation": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("/environments/test/manifest.yml", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil).Times(2)
				gomock.InOrder(
					m.iam.EXPECT().CreateECSServiceLinkedRole().Return(nil),
					// Skip deleting non-existing roles.
					m.iam.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-CFNExecutionRole")).Return(nil, errors.New("does not exist")),
					m.iam.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-EnvManagerRole")).Return(nil, errors.New("does not exist")),

					// Cleanup after created roles.
					m.iam.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-CFNExecutionRole")).Return(map[string]string{
						"copilot-application": "phonetool",
						"copilot-environment": "test",
					}, nil),
					m.iam.EXPECT().DeleteRole(gomock.Eq("phonetool-test-CFNExecutionRole")).Return(nil),
					m.iam.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-EnvManagerRole")).Return(nil, errors.New("does not exist")),
				)
				m.cfn.EXPECT().Exists("phonetool-test").Return(false, nil)
				m.deployer.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
				m.deployer.EXPECT().CreateAndRenderEnvironment(gomock.Any(), gomock.Any()).Return(errors.New("some deploy error"))
				m.appCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
			wantedErrorS: "some deploy error",
		},
		"returns error from CreateEnvironment": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{
					Name: "phonetool",
				}, nil)
				m.store.EXPECT().CreateEnvironment(gomock.Any()).Return(errors.New("some create error"))
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil).Times(2)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("/environments/test/manifest.yml", nil)
				m.iam.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				m.iam.EXPECT().ListRoleTags(gomock.Any()).
					Return(nil, errors.New("does not exist")).AnyTimes()
				m.cfn.EXPECT().Exists("phonetool-test").Return(false, nil)
				m.deployer.EXPECT().CreateAndRenderEnvironment(gomock.Any(), gomock.Any()).Return(nil)
				m.deployer.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}, nil)
				m.deployer.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
				m.appCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
			wantedErrorS: "store environment: some create error",
		},
		"success": {
			enableContainerInsights: true,
			allowDowngrade:          true,
			setupMocks: func(m *initEnvExecuteMocks) {
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.store.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(nil)
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil).Times(2)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("/environments/test/manifest.yml", nil)
				m.iam.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				m.iam.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-CFNExecutionRole")).Return(nil, errors.New("does not exist"))
				m.iam.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-EnvManagerRole")).Return(nil, errors.New("does not exist"))
				m.cfn.EXPECT().Exists("phonetool-test").Return(false, nil)
				m.deployer.EXPECT().CreateAndRenderEnvironment(gomock.Any(), gomock.Any()).Return(nil)
				m.deployer.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.deployer.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
				m.appCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
		},
		"proceed if manifest already exists": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.store.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(nil)
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil).Times(2)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("", &workspace.ErrFileExists{
					FileName: "/environments/test/manifest.yml",
				})
				m.iam.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				m.iam.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-CFNExecutionRole")).Return(nil, errors.New("does not exist"))
				m.iam.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-EnvManagerRole")).Return(nil, errors.New("does not exist"))
				m.cfn.EXPECT().Exists("phonetool-test").Return(false, nil)
				m.deployer.EXPECT().CreateAndRenderEnvironment(gomock.Any(), gomock.Any()).Return(nil)
				m.deployer.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.deployer.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
				m.appCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
		},
		"skips creating stack if environment stack already exists": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.store.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(nil)
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil).Times(2)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("/environments/test/manifest.yml", nil)
				m.iam.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				// Don't attempt to delete any roles since an environment stack already exists.
				m.iam.EXPECT().ListRoleTags(gomock.Any()).Times(0)
				m.cfn.EXPECT().Exists("phonetool-test").Return(true, nil)
				m.deployer.EXPECT().CreateAndRenderEnvironment(gomock.Any(), gomock.Any()).DoAndReturn(func(conf deploycfn.StackConfiguration, bucketARN string) error {
					require.Equal(t, conf, stack.NewBootstrapEnvStackConfig(&stack.EnvConfig{
						Name: "test",
						App: deploy.AppInformation{
							Name:                "phonetool",
							AccountPrincipalARN: "some arn",
						},
						ArtifactBucketARN:    "arn:aws:s3:::mockBucket",
						ArtifactBucketKeyARN: "mockKMS",
					}))
					require.Equal(t, bucketARN, "arn:aws:s3:::mockBucket")
					return &cloudformation.ErrStackAlreadyExists{}
				})
				m.deployer.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.deployer.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
				m.appCFN.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket:  "mockBucket",
						KMSKeyARN: "mockKMS",
					}, nil)
			},
		},
		"failed to delegate DNS (app has Domain and env and apps are different)": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool", AccountID: "1234", Domain: "amazon.com"}, nil)
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "4567"}, nil).Times(1)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("/environments/test/manifest.yml", nil)
				m.progress.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.progress.EXPECT().Stop(log.Serrorf(fmtDNSDelegationFailed, "4567"))
				m.deployer.EXPECT().DelegateDNSPermissions(gomock.Any(), "4567").Return(mockError)

			},
			wantedErrorS: "granting DNS permissions: some error",
		},
		"success with DNS Delegation (app has Domain and env and app are different)": {
			setupMocks: func(m *initEnvExecuteMocks) {
				m.appVersionGetter.EXPECT().Version().Return(mockAppVersion, nil)
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool", AccountID: "1234", Domain: "amazon.com"}, nil)
				m.store.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "4567",
					Region:    "us-west-2",
				}).Return(nil)
				m.identity.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "4567"}, nil).Times(2)
				m.manifestWriter.EXPECT().WriteEnvironmentManifest(gomock.Any(), "test").Return("/environments/test/manifest.yml", nil)
				m.iam.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				m.iam.EXPECT().ListRoleTags(gomock.Any()).
					Return(nil, errors.New("does not exist")).AnyTimes()
				m.cfn.EXPECT().Exists("phonetool-test").Return(false, nil)
				m.progress.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.progress.EXPECT().Stop(log.Ssuccessf(fmtDNSDelegationComplete, "4567"))
				m.deployer.EXPECT().DelegateDNSPermissions(gomock.Any(), "4567").Return(nil)
				m.deployer.EXPECT().CreateAndRenderEnvironment(gomock.Any(), gomock.Any()).Return(nil)
				m.deployer.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "4567",
					Region:    "us-west-2",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.deployer.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
				m.appCFN.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Domain:    "amazon.com",
				}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &initEnvExecuteMocks{
				store:            mocks.NewMockstore(ctrl),
				deployer:         mocks.NewMockdeployer(ctrl),
				identity:         mocks.NewMockidentityService(ctrl),
				progress:         mocks.NewMockprogress(ctrl),
				iam:              mocks.NewMockroleManager(ctrl),
				cfn:              mocks.NewMockstackExistChecker(ctrl),
				appCFN:           mocks.NewMockappResourcesGetter(ctrl),
				manifestWriter:   mocks.NewMockenvironmentManifestWriter(ctrl),
				appVersionGetter: mocks.NewMockversionGetter(ctrl),
			}
			tc.setupMocks(m)
			provider := sessions.ImmutableProvider()
			sess, _ := provider.DefaultWithRegion("us-west-2")

			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					name:    "test",
					appName: "phonetool",
					telemetry: telemetryVars{
						EnableContainerInsights: tc.enableContainerInsights,
					},
					allowAppDowngrade: tc.allowDowngrade,
				},
				store:       m.store,
				envDeployer: m.deployer,
				appDeployer: m.deployer,
				identity:    m.identity,
				envIdentity: m.identity,
				iam:         m.iam,
				cfn:         m.cfn,
				prog:        m.progress,
				sess:        sess,
				appCFN:      m.appCFN,
				newAppVersionGetter: func(appName string) (versionGetter, error) {
					return m.appVersionGetter, nil
				},
				manifestWriter:  m.manifestWriter,
				templateVersion: mockCurrVersion,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErrorS != "" {
				require.EqualError(t, err, tc.wantedErrorS)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInitEnvOpts_delegateDNSFromApp(t *testing.T) {
	testCases := map[string]struct {
		app            *config.Application
		expectDeployer func(m *mocks.Mockdeployer)
		expectProgress func(m *mocks.Mockprogress)
		wantedErr      string
	}{
		"should call DelegateDNSPermissions when app and env are in different accounts": {
			app: &config.Application{
				AccountID: "1234",
				Name:      "crossaccountapp",
				Domain:    "amazon.com",
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.EXPECT().Stop(log.Ssuccessf(fmtDNSDelegationComplete, "4567"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), "4567").Return(nil)
			},
		},
		"should skip updating when app and env are in same account": {
			app: &config.Application{
				AccountID: "4567",
				Name:      "crossaccountapp",
				Domain:    "amazon.com",
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(gomock.Any()).Times(0)
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		"should return errors from DelegateDNSPermissions": {
			app: &config.Application{
				AccountID: "1234",
				Name:      "crossaccountapp",
				Domain:    "amazon.com",
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.EXPECT().Stop(log.Serrorf(fmtDNSDelegationFailed, "4567"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
			},
			wantedErr: "error",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDeployer := mocks.NewMockdeployer(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)
			if tc.expectDeployer != nil {
				tc.expectDeployer(mockDeployer)
			}
			if tc.expectProgress != nil {
				tc.expectProgress(mockProgress)
			}
			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					appName: tc.app.Name,
				},
				appDeployer: mockDeployer,
				prog:        mockProgress,
			}

			// WHEN
			err := opts.delegateDNSFromApp(tc.app, "4567")

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
