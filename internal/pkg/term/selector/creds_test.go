// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/term/selector/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// mockProvider implements the AWS SDK's credentials.Provider interface.
type mockProvider struct {
	value credentials.Value
	err   error
}

func (m mockProvider) Retrieve() (credentials.Value, error) {
	if m.err != nil {
		return credentials.Value{}, m.err
	}
	return m.value, nil
}

func (m mockProvider) IsExpired() bool {
	return false
}

func TestCredsSelect_Creds(t *testing.T) {
	testCases := map[string]struct {
		inMsg  string
		inHelp string
		given  func(ctrl *gomock.Controller) *CredsSelect

		wantedErr error
	}{
		"should create a session from a named profile": {
			inMsg:  "message",
			inHelp: "help",
			given: func(ctrl *gomock.Controller) *CredsSelect {
				profile := mocks.NewMockNames(ctrl)
				profile.EXPECT().Names().Return([]string{"test", "prod"})

				prompter := mocks.NewMockPrompter(ctrl)
				prompter.EXPECT().SelectOne("message", "help", []string{
					"Enter temporary credentials",
					"[profile test]",
					"[profile prod]",
				}, gomock.Any()).Return("[profile prod]", nil)

				provider := mocks.NewMockSessionProvider(ctrl)
				provider.EXPECT().FromProfile("prod").Return(&session.Session{}, nil)

				return &CredsSelect{
					Prompt:  prompter,
					Profile: profile,
					Session: provider,
				}
			},
		},
		"should create a session from temporary credentials with masked prompt": {
			given: func(ctrl *gomock.Controller) *CredsSelect {
				profile := mocks.NewMockNames(ctrl)
				profile.EXPECT().Names().Return(nil)

				prompter := mocks.NewMockPrompter(ctrl)
				prompter.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("Enter temporary credentials", nil)

				provider := mocks.NewMockSessionProvider(ctrl)
				provider.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Credentials: credentials.NewCredentials(mockProvider{
							value: credentials.Value{
								AccessKeyID:     "11111",
								SecretAccessKey: "22222",
								SessionToken:    "33333",
							},
						}),
					},
				}, nil)

				prompter.EXPECT().Get("What's your AWS Access Key ID?", "", gomock.Any(), gomock.Any()).
					Return("****************1111", nil)
				prompter.EXPECT().Get("What's your AWS Secret Access Key?", "", gomock.Any(), gomock.Any()).
					Return("****************2222", nil)
				prompter.EXPECT().Get("What's your AWS Session Token?", "", nil, gomock.Any()).
					Return("****************3333", nil)

				provider.EXPECT().FromStaticCreds("11111", "22222", "33333").
					Return(&session.Session{}, nil)

				return &CredsSelect{
					Prompt:  prompter,
					Profile: profile,
					Session: provider,
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			sel := tc.given(ctrl)

			_, err := sel.Creds(tc.inMsg, tc.inHelp)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
