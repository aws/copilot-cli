// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"

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
}

func TestInitEnvOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inEnvName string
		inAppName string
		inDefault bool

		inVPCID      string
		inPublicIDs  []string
		inPrivateIDs []string

		inVPCCIDR     net.IPNet
		inAZs         []string
		inPublicCIDRs []string

		inProfileName     string
		inAccessKeyID     string
		inSecretAccessKey string
		inSessionToken    string

		setupMocks func(m initEnvMocks)

		wantedErrMsg string
	}{
		"valid environment creation": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",
			setupMocks: func(m initEnvMocks) {
				m.store.EXPECT().GetEnvironment("phonetool", "test-pdx").Return(nil, &config.ErrNoSuchEnvironment{})
			},
		},
		"invalid environment name": {
			inEnvName: "123env",
			inAppName: "phonetool",

			wantedErrMsg: fmt.Sprintf("environment name 123env is invalid: %s", errValueBadFormat),
		},
		"should error if environment already exists": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",

			setupMocks: func(m initEnvMocks) {
				m.store.EXPECT().GetEnvironment("phonetool", "test-pdx").Return(nil, nil)
			},
			wantedErrMsg: "environment test-pdx already exists",
		},
		"cannot specify both vpc resources importing flags and configuring flags": {
			inEnvName:     "test-pdx",
			inAppName:     "phonetool",
			inPublicCIDRs: []string{"mockCIDR"},
			inPublicIDs:   []string{"mockID", "anotherMockID"},
			inVPCCIDR: net.IPNet{
				IP:   net.IP{10, 1, 232, 0},
				Mask: net.IPMask{255, 255, 255, 0},
			},
			inVPCID: "mockID",

			setupMocks: func(m initEnvMocks) {
				m.store.EXPECT().GetEnvironment("phonetool", "test-pdx").Return(nil, &config.ErrNoSuchEnvironment{})
			},

			wantedErrMsg: "cannot specify both import vpc flags and configure vpc flags",
		},
		"cannot import or configure resources if use default flag is set": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",
			inDefault: true,
			inVPCID:   "mockID",
			setupMocks: func(m initEnvMocks) {
				m.store.EXPECT().GetEnvironment("phonetool", "test-pdx").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			wantedErrMsg: fmt.Sprintf("cannot import or configure vpc if --%s is set", defaultConfigFlag),
		},
		"should err if both profile and access key id are set": {
			inAppName:     "phonetool",
			inEnvName:     "test",
			inProfileName: "default",
			inAccessKeyID: "AKIAIOSFODNN7EXAMPLE",
			setupMocks: func(m initEnvMocks) {
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			wantedErrMsg: "cannot specify both --profile and --aws-access-key-id",
		},
		"should err if both profile and secret access key are set": {
			inAppName:         "phonetool",
			inEnvName:         "test",
			inProfileName:     "default",
			inSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			setupMocks: func(m initEnvMocks) {
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			wantedErrMsg: "cannot specify both --profile and --aws-secret-access-key",
		},
		"should err if both profile and session token are set": {
			inAppName:      "phonetool",
			inEnvName:      "test",
			inProfileName:  "default",
			inSessionToken: "verylongtoken",
			setupMocks: func(m initEnvMocks) {
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			wantedErrMsg: "cannot specify both --profile and --aws-session-token",
		},
		"should err if fewer than two private subnets are set:": {
			inVPCID:      "mockID",
			inPublicIDs:  []string{"mockID", "anotherMockID"},
			inPrivateIDs: []string{"mockID"},

			wantedErrMsg: "at least two private subnets must be imported",
		},
		"should err if fewer than two availability zones are provided": {
			inAZs: []string{"us-east-1a"},

			wantedErrMsg: "at least two availability zones must be provided to enable Load Balancing",
		},
		"invalid VPC resource import (no VPC, 1 public, 2 private)": {
			inPublicIDs:  []string{"mockID"},
			inPrivateIDs: []string{"mockID", "anotherMockID"},

			wantedErrMsg: "at least two public subnets must be imported to enable Load Balancing",
		},
		"valid VPC resource import (0 public, 3 private)": {
			inVPCID:      "mockID",
			inPublicIDs:  []string{},
			inPrivateIDs: []string{"mockID", "anotherMockID", "yetAnotherMockID"},
		},
		"valid VPC resource import (3 public, 2 private)": {
			inVPCID:      "mockID",
			inPublicIDs:  []string{"mockID", "anotherMockID", "yetAnotherMockID"},
			inPrivateIDs: []string{"mockID", "anotherMockID"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := initEnvMocks{
				store: mocks.NewMockstore(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}

			// GIVEN
			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					name:          tc.inEnvName,
					defaultConfig: tc.inDefault,
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
				store: m.store,
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
		inAppName       string
		inEnv           string
		inProfile       string
		inTempCreds     tempCredsVars
		inRegion        string
		inDefault       bool
		inImportVPCVars importVPCVars
		inAdjustVPCVars adjustVPCVars

		setupMocks func(mocks initEnvMocks)

		wantedError error
	}{
		"should prompt for app if currently not under a workspace and none is specified": {
			inAppName: "",
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inDefault: true,

			setupMocks: func(m initEnvMocks) {
				m.selApp.EXPECT().
					Application(envInitAppNamePrompt, envInitAppNameHelpPrompt).
					Return(mockApp, nil)
				m.sessProvider.EXPECT().FromProfile(mockProfile).Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
			},

			wantedError: nil,
		},
		"fail to get app name": {
			inAppName: "",

			setupMocks: func(m initEnvMocks) {
				m.selApp.EXPECT().
					Application(envInitAppNamePrompt, envInitAppNameHelpPrompt).
					Return("", mockErr)
			},

			wantedError: fmt.Errorf("ask for application: some error"),
		},
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
				)
			},
			wantedError: errors.New("environment test already exists"),
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
			}

			tc.setupMocks(mocks)
			// GIVEN
			addEnv := &initEnvOpts{
				initEnvVars: initEnvVars{
					appName:       tc.inAppName,
					name:          tc.inEnv,
					profile:       tc.inProfile,
					tempCreds:     tc.inTempCreds,
					region:        tc.inRegion,
					defaultConfig: tc.inDefault,
					adjustVPC:     tc.inAdjustVPCVars,
					importVPC:     tc.inImportVPCVars,
				},
				sessProvider: mocks.sessProvider,
				selVPC:       mocks.selVPC,
				selCreds:     mocks.selCreds,
				ec2Client:    mocks.ec2Client,
				prompt:       mocks.prompt,
				selApp:       mocks.selApp,
				store:        mocks.store,
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

func TestInitEnvOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		enableContainerInsights bool

		expectStore             func(m *mocks.Mockstore)
		expectDeployer          func(m *mocks.Mockdeployer)
		expectIdentity          func(m *mocks.MockidentityService)
		expectProgress          func(m *mocks.Mockprogress)
		expectIAM               func(m *mocks.MockroleManager)
		expectCFN               func(m *mocks.MockstackExistChecker)
		expectAppCFN            func(m *mocks.MockappResourcesGetter)
		expectResourcesUploader func(m *mocks.MockcustomResourcesUploader)

		wantedErrorS string
	}{
		"returns app exists error": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))
			},

			wantedErrorS: "some error",
		},
		"returns identity get error": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{}, errors.New("some identity error"))
			},
			wantedErrorS: "get identity: some identity error",
		},
		"failed to create stack set instance": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateEnvironment(gomock.Any()).Times(0)
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil)
			},
			expectIAM: func(m *mocks.MockroleManager) {
				m.EXPECT().CreateECSServiceLinkedRole().Return(nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "us-west-2", "phonetool"))
				m.EXPECT().Stop(log.Serrorf(fmtAddEnvToAppFailed, "1234", "us-west-2", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().AddEnvToApp(&deploycfn.AddEnvToAppOpts{
					App:          &config.Application{Name: "phonetool"},
					EnvName:      "test",
					EnvAccountID: "1234",
					EnvRegion:    "us-west-2",
				}).Return(errors.New("some cfn error"))
			},
			wantedErrorS: "deploy env test to application phonetool: some cfn error",
		},
		"errors cannot get app resources by region": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "us-west-2", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "us-west-2", "phonetool"))
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil)
			},
			expectIAM: func(m *mocks.MockroleManager) {
				m.EXPECT().CreateECSServiceLinkedRole().Return(nil)
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
			},
			expectAppCFN: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(nil, errors.New("some error"))
			},
			wantedErrorS: "get app resources: some error",
		},
		"errors cannot read env lambdas": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "us-west-2", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "us-west-2", "phonetool"))
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil)
			},
			expectIAM: func(m *mocks.MockroleManager) {
				m.EXPECT().CreateECSServiceLinkedRole().Return(nil)
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
			},
			expectAppCFN: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
			expectResourcesUploader: func(m *mocks.MockcustomResourcesUploader) {
				m.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(nil, fmt.Errorf("some error"))
			},
			wantedErrorS: "upload custom resources to bucket mockBucket: some error",
		},
		"deletes retained IAM roles if environment stack fails creation": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "us-west-2", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "us-west-2", "phonetool"))
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil).Times(2)
			},
			expectIAM: func(m *mocks.MockroleManager) {
				gomock.InOrder(
					m.EXPECT().CreateECSServiceLinkedRole().Return(nil),
					// Skip deleting non-existing roles.
					m.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-CFNExecutionRole")).Return(nil, errors.New("does not exist")),
					m.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-EnvManagerRole")).Return(nil, errors.New("does not exist")),

					// Cleanup after created roles.
					m.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-CFNExecutionRole")).Return(map[string]string{
						"copilot-application": "phonetool",
						"copilot-environment": "test",
					}, nil),
					m.EXPECT().DeleteRole(gomock.Eq("phonetool-test-CFNExecutionRole")).Return(nil),
					m.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-EnvManagerRole")).Return(nil, errors.New("does not exist")),
				)
			},
			expectCFN: func(m *mocks.MockstackExistChecker) {
				m.EXPECT().Exists("phonetool-test").Return(false, nil)
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
				m.EXPECT().DeployAndRenderEnvironment(gomock.Any(), gomock.Any()).Return(errors.New("some deploy error"))
			},
			expectAppCFN: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
			expectResourcesUploader: func(m *mocks.MockcustomResourcesUploader) {
				m.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(nil, nil)
			},
			wantedErrorS: "some deploy error",
		},
		"returns error from CreateEnvironment": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{
					Name: "phonetool",
				}, nil)
				m.EXPECT().CreateEnvironment(gomock.Any()).Return(errors.New("some create error"))
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil).Times(2)
			},
			expectIAM: func(m *mocks.MockroleManager) {
				m.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				m.EXPECT().ListRoleTags(gomock.Any()).
					Return(nil, errors.New("does not exist")).AnyTimes()
			},
			expectCFN: func(m *mocks.MockstackExistChecker) {
				m.EXPECT().Exists("phonetool-test").Return(false, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "us-west-2", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "us-west-2", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployAndRenderEnvironment(gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
			},
			expectAppCFN: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
			expectResourcesUploader: func(m *mocks.MockcustomResourcesUploader) {
				m.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(nil, nil)
			},
			wantedErrorS: "store environment: some create error",
		},
		"success": {
			enableContainerInsights: true,

			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
					Telemetry: &config.Telemetry{
						EnableContainerInsights: true,
					},
				}).Return(nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil).Times(2)
			},
			expectIAM: func(m *mocks.MockroleManager) {
				m.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				m.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-CFNExecutionRole")).Return(nil, errors.New("does not exist"))
				m.EXPECT().ListRoleTags(gomock.Eq("phonetool-test-EnvManagerRole")).Return(nil, errors.New("does not exist"))
			},
			expectCFN: func(m *mocks.MockstackExistChecker) {
				m.EXPECT().Exists("phonetool-test").Return(false, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "us-west-2", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "us-west-2", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployAndRenderEnvironment(gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
			},
			expectAppCFN: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
			expectResourcesUploader: func(m *mocks.MockcustomResourcesUploader) {
				m.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(nil, nil)
			},
		},
		"skips creating stack if environment stack already exists": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
					Telemetry: &config.Telemetry{
						EnableContainerInsights: false,
					},
				}).Return(nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "1234"}, nil).Times(2)
			},
			expectIAM: func(m *mocks.MockroleManager) {
				m.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				// Don't attempt to delete any roles since an environment stack already exists.
				m.EXPECT().ListRoleTags(gomock.Any()).Times(0)
			},
			expectCFN: func(m *mocks.MockstackExistChecker) {
				m.EXPECT().Exists("phonetool-test").Return(true, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "us-west-2", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "us-west-2", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployAndRenderEnvironment(gomock.Any(), &deploy.CreateEnvironmentInput{
					Name: "test",
					App: deploy.AppInformation{
						Name:                "phonetool",
						AccountPrincipalARN: "some arn",
					},
					CustomResourcesURLs: map[string]string{"mockCustomResource": "mockURL"},
					Telemetry: &config.Telemetry{
						EnableContainerInsights: false,
					},
					Version:              deploy.LatestEnvTemplateVersion,
					ArtifactBucketARN:    "arn:aws:s3:::mockBucket",
					ArtifactBucketKeyARN: "mockKMS",
				}).Return(&cloudformation.ErrStackAlreadyExists{})
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
			},
			expectAppCFN: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{Name: "phonetool"}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket:  "mockBucket",
						KMSKeyARN: "mockKMS",
					}, nil)
			},
			expectResourcesUploader: func(m *mocks.MockcustomResourcesUploader) {
				m.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(map[string]string{"mockCustomResource": "mockURL"}, nil)
			},
		},
		"failed to delegate DNS (app has Domain and env and apps are different)": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool", AccountID: "1234", Domain: "amazon.com"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "4567"}, nil).Times(1)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.EXPECT().Stop(log.Serrorf(fmtDNSDelegationFailed, "4567"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), "4567").Return(errors.New("some error"))
			},
			wantedErrorS: "granting DNS permissions: some error",
		},
		"success with DNS Delegation (app has Domain and env and app are different)": {
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool", AccountID: "1234", Domain: "amazon.com"}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "4567",
					Region:    "us-west-2",
					Telemetry: &config.Telemetry{
						EnableContainerInsights: false,
					},
				}).Return(nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "4567"}, nil).Times(2)
			},
			expectIAM: func(m *mocks.MockroleManager) {
				m.EXPECT().CreateECSServiceLinkedRole().Return(nil)
				m.EXPECT().ListRoleTags(gomock.Any()).
					Return(nil, errors.New("does not exist")).AnyTimes()
			},
			expectCFN: func(m *mocks.MockstackExistChecker) {
				m.EXPECT().Exists("phonetool-test").Return(false, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.EXPECT().Stop(log.Ssuccessf(fmtDNSDelegationComplete, "4567"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "4567", "us-west-2", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "4567", "us-west-2", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), "4567").Return(nil)
				m.EXPECT().DeployAndRenderEnvironment(gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "4567",
					Region:    "us-west-2",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any()).Return(nil)
			},
			expectAppCFN: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Domain:    "amazon.com",
				}, "us-west-2").
					Return(&stack.AppRegionalResources{
						S3Bucket: "mockBucket",
					}, nil)
			},
			expectResourcesUploader: func(m *mocks.MockcustomResourcesUploader) {
				m.EXPECT().UploadEnvironmentCustomResources(gomock.Any()).Return(nil, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)
			mockDeployer := mocks.NewMockdeployer(ctrl)
			mockIdentity := mocks.NewMockidentityService(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)
			mockAppCFN := mocks.NewMockappResourcesGetter(ctrl)
			mockIAM := mocks.NewMockroleManager(ctrl)
			mockCFN := mocks.NewMockstackExistChecker(ctrl)
			mockResourcesUploader := mocks.NewMockcustomResourcesUploader(ctrl)
			mockUploader := mocks.NewMockuploader(ctrl)
			if tc.expectStore != nil {
				tc.expectStore(mockStore)
			}
			if tc.expectDeployer != nil {
				tc.expectDeployer(mockDeployer)
			}
			if tc.expectIdentity != nil {
				tc.expectIdentity(mockIdentity)
			}
			if tc.expectIAM != nil {
				tc.expectIAM(mockIAM)
			}
			if tc.expectCFN != nil {
				tc.expectCFN(mockCFN)
			}
			if tc.expectProgress != nil {
				tc.expectProgress(mockProgress)
			}
			if tc.expectAppCFN != nil {
				tc.expectAppCFN(mockAppCFN)
			}
			if tc.expectResourcesUploader != nil {
				tc.expectResourcesUploader(mockResourcesUploader)
			}

			provider := sessions.NewProvider()
			sess, _ := provider.DefaultWithRegion("us-west-2")

			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					name:    "test",
					appName: "phonetool",
					telemetry: telemetryVars{
						EnableContainerInsights: tc.enableContainerInsights,
					},
				},
				store:       mockStore,
				envDeployer: mockDeployer,
				appDeployer: mockDeployer,
				identity:    mockIdentity,
				envIdentity: mockIdentity,
				iam:         mockIAM,
				cfn:         mockCFN,
				prog:        mockProgress,
				sess:        sess,
				appCFN:      mockAppCFN,
				uploader:    mockResourcesUploader,
				newS3: func(region string) (uploader, error) {
					return mockUploader, nil
				},
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
