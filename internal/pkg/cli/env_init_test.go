// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
)

type initEnvMocks struct {
	sessProvider *mocks.MocksessionProvider
	prompt       *mocks.Mockprompter
	selVPC       *mocks.Mockec2Selector
	selCreds     *mocks.MockcredsSelector
	config       *mocks.MockprofileNames
	ec2Client    *mocks.Mockec2Client
}

func TestInitEnvOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inEnvName     string
		inAppName     string
		inDefault     bool
		inVPCID       string
		inPublicIDs   []string
		inVPCCIDR     net.IPNet
		inPublicCIDRs []string

		inProfileName     string
		inAccessKeyID     string
		inSecretAccessKey string
		inSessionToken    string

		wantedErrMsg string
	}{
		"valid environment creation": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",
		},
		"invalid environment name": {
			inEnvName: "123env",
			inAppName: "phonetool",

			wantedErrMsg: fmt.Sprintf("environment name 123env is invalid: %s", errValueBadFormat),
		},
		"new workspace": {
			inEnvName: "test-pdx",
			inAppName: "",

			wantedErrMsg: "no application found: run `app init` or `cd` into your workspace please",
		},
		"cannot specify both vpc resources importing flags and configuing flags": {
			inEnvName:     "test-pdx",
			inAppName:     "phonetool",
			inPublicCIDRs: []string{"mockCIDR"},
			inPublicIDs:   []string{"mockID"},
			inVPCCIDR: net.IPNet{
				IP:   net.IP{10, 1, 232, 0},
				Mask: net.IPMask{255, 255, 255, 0},
			},
			inVPCID: "mockID",

			wantedErrMsg: "cannot specify both import vpc flags and configure vpc flags",
		},
		"cannot import or configure resources if use default flag is set": {
			inEnvName: "test-pdx",
			inAppName: "phonetool",
			inDefault: true,
			inVPCID:   "mockID",

			wantedErrMsg: fmt.Sprintf("cannot import or configure vpc if --%s is set", defaultConfigFlag),
		},
		"should err if both profile and access key id are set": {
			inAppName:     "phonetool",
			inEnvName:     "test",
			inProfileName: "default",
			inAccessKeyID: "AKIAIOSFODNN7EXAMPLE",

			wantedErrMsg: "cannot specify both --profile and --aws-access-key-id",
		},
		"should err if both profile and secret access key are set": {
			inAppName:         "phonetool",
			inEnvName:         "test",
			inProfileName:     "default",
			inSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",

			wantedErrMsg: "cannot specify both --profile and --aws-secret-access-key",
		},
		"should err if both profile and session token are set": {
			inAppName:      "phonetool",
			inEnvName:      "test",
			inProfileName:  "default",
			inSessionToken: "verylongtoken",

			wantedErrMsg: "cannot specify both --profile and --aws-session-token",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					Name:          tc.inEnvName,
					DefaultConfig: tc.inDefault,
					AdjustVPC: adjustVPCVars{
						PublicSubnetCIDRs: tc.inPublicCIDRs,
						CIDR:              tc.inVPCCIDR,
					},
					ImportVPC: importVPCVars{
						PublicSubnetIDs: tc.inPublicIDs,
						ID:              tc.inVPCID,
					},
					GlobalOpts: &GlobalOpts{appName: tc.inAppName},
					Profile:    tc.inProfileName,
					TempCreds: tempCredsVars{
						AccessKeyID:     tc.inAccessKeyID,
						SecretAccessKey: tc.inSecretAccessKey,
						SessionToken:    tc.inSessionToken,
					},
				},
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

	testCases := map[string]struct {
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
		"fail to get env name": {
			setupMocks: func(m initEnvMocks) {
				gomock.InOrder(
					m.prompt.EXPECT().
						Get(envInitNamePrompt, envInitNameHelpPrompt, gomock.Any()).
						Return("", mockErr),
				)
			},
			wantedError: fmt.Errorf("get environment name: some error"),
		},
		"should create a session from a named profile if flag is provided": {
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
			inEnv: mockEnv,
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
			inEnv:     mockEnv,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.selCreds.EXPECT().Creds("Which credentials would you like to use to create test?", gomock.Any()).Return(mockSession, nil)
			},
		},
		"should prompt for region if user configuration does not have one": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(&session.Session{
					Config: &aws.Config{},
				}, nil)
				m.prompt.EXPECT().Get("Which region?", gomock.Any(), nil, gomock.Any()).Return("us-west-2", nil)
			},
		},
		"should skip prompting for region if flag is provided": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inRegion:  mockRegion,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(&session.Session{
					Config: &aws.Config{},
				}, nil)
				m.prompt.EXPECT().Get("Which region?", gomock.Any(), nil, gomock.Any()).Times(0)
			},
		},
		"should not prompt for configuring environment if default config flag is true": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inDefault: true,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any()).Times(0)
			},
		},
		"fail to select whether to adjust or import resources": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return("", mockErr)
			},
			wantedError: fmt.Errorf("select adjusting or importing resources: some error"),
		},
		"success with no custom resources": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitDefaultConfigSelectOption, nil)
			},
		},
		"fail to select VPC": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("", mockErr)
			},
			wantedError: fmt.Errorf("select VPC: some error"),
		},
		"fail to check if VPC has DNS support": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(false, mockErr)
			},
			wantedError: fmt.Errorf("check if VPC mockVPC has DNS support enabled: some error"),
		},
		"fail to import VPC has no DNS support": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(false, nil)
			},
			wantedError: fmt.Errorf("VPC mockVPC has no DNS support enabled"),
		},
		"fail to select public subnets": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().PublicSubnets(envInitPublicSubnetsSelectPrompt, "", "mockVPC").
					Return(nil, mockErr)
			},
			wantedError: fmt.Errorf("select public subnets: some error"),
		},
		"fail to select private subnets": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().PublicSubnets(envInitPublicSubnetsSelectPrompt, "", "mockVPC").
					Return([]string{"mockPublicSubnet"}, nil)
				m.selVPC.EXPECT().PrivateSubnets(envInitPrivateSubnetsSelectPrompt, "", "mockVPC").
					Return(nil, mockErr)
			},
			wantedError: fmt.Errorf("select private subnets: some error"),
		},
		"success with importing env resources with no flags": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitImportEnvResourcesSelectOption, nil)
				m.selVPC.EXPECT().VPC(envInitVPCSelectPrompt, "").Return("mockVPC", nil)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPC").Return(true, nil)
				m.selVPC.EXPECT().PublicSubnets(envInitPublicSubnetsSelectPrompt, "", "mockVPC").
					Return([]string{"mockPublicSubnet"}, nil)
				m.selVPC.EXPECT().PrivateSubnets(envInitPrivateSubnetsSelectPrompt, "", "mockVPC").
					Return([]string{"mockPrivateSubnet"}, nil)
			},
		},
		"success with importing env resources with flags": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inImportVPCVars: importVPCVars{
				ID:               "mockVPCID",
				PrivateSubnetIDs: []string{"mockPrivateSubnetID"},
				PublicSubnetIDs:  []string{"mockPublicSubnetID"},
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any()).Times(0)
				m.ec2Client.EXPECT().HasDNSSupport("mockVPCID").Return(true, nil)
			},
		},
		"fail to get VPC CIDR": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, envInitVPCCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return("", mockErr)
			},
			wantedError: fmt.Errorf("get VPC CIDR: some error"),
		},
		"fail to get public subnet CIDRs": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, envInitVPCCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return(mockVPCCIDR, nil)
				m.prompt.EXPECT().Get(envInitPublicCIDRPrompt, envInitPublicCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return("", mockErr)
			},
			wantedError: fmt.Errorf("get public subnet CIDRs: some error"),
		},
		"fail to get private subnet CIDRs": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, envInitVPCCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return(mockVPCCIDR, nil)
				m.prompt.EXPECT().Get(envInitPublicCIDRPrompt, envInitPublicCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return(mockSubnetCIDRs, nil)
				m.prompt.EXPECT().Get(envInitPrivateCIDRPrompt, envInitPrivateCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return("", mockErr)
			},
			wantedError: fmt.Errorf("get private subnet CIDRs: some error"),
		},
		"success with adjusting default env config with no flags": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, "", envInitCustomizedEnvTypes).
					Return(envInitAdjustEnvResourcesSelectOption, nil)
				m.prompt.EXPECT().Get(envInitVPCCIDRPrompt, envInitVPCCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return(mockVPCCIDR, nil)
				m.prompt.EXPECT().Get(envInitPublicCIDRPrompt, envInitPublicCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return(mockSubnetCIDRs, nil)
				m.prompt.EXPECT().Get(envInitPrivateCIDRPrompt, envInitPrivateCIDRPromptHelp, gomock.Any(), gomock.Any()).
					Return(mockSubnetCIDRs, nil)
			},
		},
		"success with adjusting default env config with flags": {
			inEnv:     mockEnv,
			inProfile: mockProfile,
			inAdjustVPCVars: adjustVPCVars{
				CIDR: net.IPNet{
					IP:   net.IP{10, 1, 232, 0},
					Mask: net.IPMask{255, 255, 255, 0},
				},
				PrivateSubnetCIDRs: []string{"mockPrivateCIDR"},
				PublicSubnetCIDRs:  []string{"mockPublicCIDR"},
			},
			setupMocks: func(m initEnvMocks) {
				m.sessProvider.EXPECT().FromProfile(gomock.Any()).Return(mockSession, nil)
				m.prompt.EXPECT().SelectOne(envInitDefaultEnvConfirmPrompt, gomock.Any(), gomock.Any()).Times(0)
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
				config:       mocks.NewMockprofileNames(ctrl),
				selVPC:       mocks.NewMockec2Selector(ctrl),
				selCreds:     mocks.NewMockcredsSelector(ctrl),
				ec2Client:    mocks.NewMockec2Client(ctrl),
			}

			tc.setupMocks(mocks)
			// GIVEN
			addEnv := &initEnvOpts{
				initEnvVars: initEnvVars{
					Name:          tc.inEnv,
					Profile:       tc.inProfile,
					TempCreds:     tc.inTempCreds,
					Region:        tc.inRegion,
					DefaultConfig: tc.inDefault,
					AdjustVPC:     tc.inAdjustVPCVars,
					ImportVPC:     tc.inImportVPCVars,
					GlobalOpts: &GlobalOpts{
						prompt: mocks.prompt,
					},
				},
				sessProvider: mocks.sessProvider,
				selVPC:       mocks.selVPC,
				selCreds:     mocks.selCreds,
				ec2Client:    mocks.ec2Client,
			}

			// WHEN
			err := addEnv.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, mockEnv, addEnv.Name, "expected environment names to match")
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestInitEnvOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inProd    bool

		expectstore    func(m *mocks.Mockstore)
		expectDeployer func(m *mocks.Mockdeployer)
		expectIdentity func(m *mocks.MockidentityService)
		expectProgress func(m *mocks.Mockprogress)

		wantedErrorS string
	}{
		"returns app exists error": {
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))
			},

			wantedErrorS: "some error",
		},
		"returns identity get error": {
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{}, errors.New("some identity error"))
			},
			wantedErrorS: "get identity: some identity error",
		},
		"errors if environment change set cannot be accepted": {
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Stop(log.Serrorf(fmtDeployEnvFailed, "test"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(errors.New("some deploy error"))
			},
			wantedErrorS: "some deploy error",
		},
		"streams failed events": {
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Events([]termprogress.TabRow{
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textVPC, termprogress.StatusFailed)),
					termprogress.TabRow(fmt.Sprintf("  %s\t", "some reason")),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textInternetGateway, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textPublicSubnets, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textPrivateSubnets, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textRouteTables, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textECSCluster, termprogress.StatusInProgress)),
					termprogress.TabRow(fmt.Sprintf("%s\t[%s]", textALB, termprogress.StatusInProgress)),
				})
				m.EXPECT().Stop(log.Serrorf(fmtStreamEnvFailed, "test"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				events <- []deploy.ResourceEvent{
					{
						Resource: deploy.Resource{
							LogicalName: "VPC",
							Type:        "AWS::EC2::VPC",
						},
						Status:       "CREATE_FAILED",
						StatusReason: "some reason",
					},
				}
				responses <- deploy.CreateEnvironmentResponse{
					Err: errors.New("some stream error"),
				}
				close(events)
				close(responses)
			},
			wantedErrorS: "some stream error",
		},
		"failed to get environment stack": {
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateEnvironment(gomock.Any()).Times(0)
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				env := &config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}
				responses <- deploy.CreateEnvironmentResponse{
					Env: env,
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, errors.New("some error"))
			},
			wantedErrorS: "get environment struct for test: some error",
		},
		"failed to create stack set instance": {
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateEnvironment(gomock.Any()).Times(0)
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Serrorf(fmtAddEnvToAppFailed, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				env := &config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}
				responses <- deploy.CreateEnvironmentResponse{
					Env: env,
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(&config.Application{Name: "phonetool"}, env).Return(errors.New("some cfn error"))
			},
			wantedErrorS: "deploy env test to application phonetool: some cfn error",
		},
		"returns error from CreateEnvironment": {
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{
					Name: "phonetool",
				}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(errors.New("some create error"))
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &config.Environment{
						App:       "phonetool",
						Name:      "test",
						AccountID: "1234",
						Region:    "mars-1",
					},
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantedErrorS: "store environment: some create error",
		},
		"success": {
			inAppName: "phonetool",
			inEnvName: "test",
			inProd:    true,

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Prod:      true,
					Region:    "mars-1",
				}).Return(nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &config.Environment{
						App:       "phonetool",
						Name:      "test",
						AccountID: "1234",
						Prod:      true,
						Region:    "mars-1",
					},
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					Prod:      false,
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		"skips creating stack if environment stack already exists": {
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtDeployEnvComplete, "test", "phonetool"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DeployEnvironment(&deploy.CreateEnvironmentInput{
					Name:                     "test",
					AppName:                  "phonetool",
					ToolsAccountPrincipalARN: "some arn",
				}).Return(&cloudformation.ErrStackAlreadyExists{})
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		"failed to delegate DNS (app has Domain and env and apps are different)": {
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
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
			inAppName: "phonetool",
			inEnvName: "test",

			expectstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool", AccountID: "1234", Domain: "amazon.com"}, nil)
				m.EXPECT().CreateEnvironment(&config.Environment{
					App:       "phonetool",
					Name:      "test",
					AccountID: "1234",
					Region:    "mars-1",
				}).Return(nil)
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{RootUserARN: "some arn", Account: "4567"}, nil).Times(2)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtDNSDelegationStart, "4567"))
				m.EXPECT().Stop(log.Ssuccessf(fmtDNSDelegationComplete, "4567"))
				m.EXPECT().Start(fmt.Sprintf(fmtDeployEnvStart, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtStreamEnvStart, "test"))
				m.EXPECT().Stop(log.Ssuccessf(fmtStreamEnvComplete, "test"))
				m.EXPECT().Start(fmt.Sprintf(fmtAddEnvToAppStart, "1234", "mars-1", "phonetool"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddEnvToAppComplete, "1234", "mars-1", "phonetool"))
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), "4567").Return(nil)
				m.EXPECT().DeployEnvironment(gomock.Any()).Return(nil)
				events := make(chan []deploy.ResourceEvent, 1)
				responses := make(chan deploy.CreateEnvironmentResponse, 1)
				m.EXPECT().StreamEnvironmentCreation(gomock.Any()).Return(events, responses)
				responses <- deploy.CreateEnvironmentResponse{
					Env: &config.Environment{
						App:       "phonetool",
						Name:      "test",
						AccountID: "1234",
						Region:    "mars-1",
					},
					Err: nil,
				}
				close(events)
				close(responses)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{
					AccountID: "1234",
					Region:    "mars-1",
					Name:      "test",
					App:       "phonetool",
				}, nil)
				m.EXPECT().AddEnvToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockDeployer := mocks.NewMockdeployer(ctrl)
			mockIdentity := mocks.NewMockidentityService(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)
			if tc.expectstore != nil {
				tc.expectstore(mockstore)
			}
			if tc.expectDeployer != nil {
				tc.expectDeployer(mockDeployer)
			}
			if tc.expectIdentity != nil {
				tc.expectIdentity(mockIdentity)
			}
			if tc.expectProgress != nil {
				tc.expectProgress(mockProgress)
			}

			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					Name:         tc.inEnvName,
					GlobalOpts:   &GlobalOpts{appName: tc.inAppName},
					IsProduction: tc.inProd,
				},
				store:       mockstore,
				envDeployer: mockDeployer,
				appDeployer: mockDeployer,
				identity:    mockIdentity,
				envIdentity: mockIdentity,
				prog:        mockProgress,
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
		expectIdentity func(m *mocks.MockidentityService)
		expectProgress func(m *mocks.Mockprogress)
		wantedErr      string
	}{
		"should call DelegateDNSPermissions when app and env are in different accounts": {
			app: &config.Application{
				AccountID: "1234",
				Name:      "crossaccountapp",
				Domain:    "amazon.com",
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{Account: "4567"}, nil)
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
				AccountID: "1234",
				Name:      "crossaccountapp",
				Domain:    "amazon.com",
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{Account: "1234"}, nil)
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(gomock.Any()).Times(0)
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		"should return errors from identity": {
			app: &config.Application{
				AccountID: "1234",
				Name:      "crossaccountapp",
				Domain:    "amazon.com",
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{}, fmt.Errorf("error"))
			},
			expectProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(gomock.Any()).Times(0)
			},
			expectDeployer: func(m *mocks.Mockdeployer) {
				m.EXPECT().DelegateDNSPermissions(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "getting environment account ID for DNS Delegation: error",
		},
		"should return errors from DelegateDNSPermissions": {
			app: &config.Application{
				AccountID: "1234",
				Name:      "crossaccountapp",
				Domain:    "amazon.com",
			},
			expectIdentity: func(m *mocks.MockidentityService) {
				m.EXPECT().Get().Return(identity.Caller{Account: "4567"}, nil)
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
			mockIdentity := mocks.NewMockidentityService(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)
			if tc.expectDeployer != nil {
				tc.expectDeployer(mockDeployer)
			}
			if tc.expectIdentity != nil {
				tc.expectIdentity(mockIdentity)
			}
			if tc.expectProgress != nil {
				tc.expectProgress(mockProgress)
			}
			opts := &initEnvOpts{
				initEnvVars: initEnvVars{
					GlobalOpts: &GlobalOpts{appName: tc.app.Name},
				},
				envIdentity: mockIdentity,
				appDeployer: mockDeployer,
				prog:        mockProgress,
			}

			// WHEN
			err := opts.delegateDNSFromApp(tc.app)

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
