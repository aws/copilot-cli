// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnvUpgradeOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		given     func(ctrl *gomock.Controller) *envUpgradeOpts
		wantedErr error
	}{
		"should not error if the environment exists and a name is provided": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				m := mocks.NewMockenvironmentStore(ctrl)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, nil)

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store: m,
				}
			},
		},
		"should throw a config.ErrNoSuchEnvironment if the environment is not found": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				m := mocks.NewMockenvironmentStore(ctrl)
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(nil, &config.ErrNoSuchEnvironment{
					ApplicationName: "phonetool",
					EnvironmentName: "test",
				})

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store: m,
				}
			},
			wantedErr: &config.ErrNoSuchEnvironment{
				ApplicationName: "phonetool",
				EnvironmentName: "test",
			},
		},
		"should throw a wrapped error on unexpected config failure": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				m := mocks.NewMockenvironmentStore(ctrl)
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))

				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
					},
					store: m,
				}
			},
			wantedErr: errors.New("get environment test configuration from application phonetool: some error"),
		},
		"should not allow --all and --name": {
			given: func(ctrl *gomock.Controller) *envUpgradeOpts {
				return &envUpgradeOpts{
					envUpgradeVars: envUpgradeVars{
						appName: "phonetool",
						name:    "test",
						all:     true,
					},
				}
			},
			wantedErr: errors.New("cannot specify both --all and --name flags"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := tc.given(ctrl)

			err := opts.Validate()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
