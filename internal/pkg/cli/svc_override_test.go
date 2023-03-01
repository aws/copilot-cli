// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestOverrideSvcOpts_Validate(t *testing.T) {
	t.Run("validate application", func(t *testing.T) {
		testCases := map[string]struct {
			appName   string
			initMocks func(ctrl *gomock.Controller, cmd *overrideSvcOpts)

			wanted error
		}{
			"return an error if the workspace has no application associated with": {
				wanted: errNoAppInWorkspace,
			},
			"return a wrapped error if the workspace's application cannot be fetched from the Config Store": {
				appName: "demo",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideSvcOpts) {
					mockSSM := mocks.NewMockstore(ctrl)
					mockSSM.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))
					cmd.cfgStore = mockSSM
				},
				wanted: errors.New(`get application "demo" configuration: some error`),
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				in := overrideVars{appName: tc.appName}
				cmd := &overrideSvcOpts{overrideVars: in}
				if tc.initMocks != nil {
					tc.initMocks(ctrl, cmd)
				}

				// WHEN
				err := cmd.Validate()

				// THEN
				if tc.wanted != nil {
					require.EqualError(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
	t.Run("validate CDK language", func(t *testing.T) {
		testCases := map[string]struct {
			lang   string
			wanted error
		}{
			"return an error when an unknown language is selected": {
				lang:   "python",
				wanted: errors.New(`"python" is not a valid CDK language: must be one of: "typescript"`),
			},
			"typescript is a valid CDK language": {
				lang: "typescript",
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				mockSSM := mocks.NewMockstore(ctrl)
				mockSSM.EXPECT().GetApplication(gomock.Any()).Return(nil, nil)

				in := overrideVars{cdkLang: tc.lang, appName: "demo"}
				cmd := &overrideSvcOpts{
					overrideVars: in,
					cfgStore:     mockSSM,
				}

				// WHEN
				err := cmd.Validate()

				// THEN
				if tc.wanted != nil {
					require.EqualError(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}
