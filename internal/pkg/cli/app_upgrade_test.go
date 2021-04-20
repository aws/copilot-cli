// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAppUpgradeOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		given     func(ctrl *gomock.Controller) *appUpgradeOpts
		wantedErr error
	}{
		"should return error if fail to get template version": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockVersionGetter := mocks.NewMockversionGetter(ctrl)
				mockVersionGetter.EXPECT().Version().Return("", errors.New("some error"))

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					versionGetter: mockVersionGetter,
				}
			},
			wantedErr: fmt.Errorf("get template version of application phonetool: some error"),
		},
		"should return if app is up-to-date": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockVersionGetter := mocks.NewMockversionGetter(ctrl)
				mockVersionGetter.EXPECT().Version().Return(deploy.LatestAppTemplateVersion, nil)

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					versionGetter: mockVersionGetter,
				}
			},
		},
		"should return error if fail to get application": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockVersionGetter := mocks.NewMockversionGetter(ctrl)
				mockVersionGetter.EXPECT().Version().Return(deploy.LegacyAppTemplateVersion, nil)

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					versionGetter: mockVersionGetter,
					store:         mockStore,
				}
			},
			wantedErr: fmt.Errorf("get application phonetool: some error"),
		},
		"should return error if fail to get identity": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockVersionGetter := mocks.NewMockversionGetter(ctrl)
				mockVersionGetter.EXPECT().Version().Return(deploy.LegacyAppTemplateVersion, nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				mockIdentity := mocks.NewMockidentityService(ctrl)
				mockIdentity.EXPECT().Get().Return(identity.Caller{}, errors.New("some error"))

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					versionGetter: mockVersionGetter,
					identity:      mockIdentity,
					store:         mockStore,
					prog:          mockProg,
				}
			},
			wantedErr: fmt.Errorf("get identity: some error"),
		},
		"should return error if fail to upgrade application": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockVersionGetter := mocks.NewMockversionGetter(ctrl)
				mockVersionGetter.EXPECT().Version().Return(deploy.LegacyAppTemplateVersion, nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				mockIdentity := mocks.NewMockidentityService(ctrl)
				mockIdentity.EXPECT().Get().Return(identity.Caller{Account: "1234"}, nil)

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)

				mockUpgrader := mocks.NewMockappUpgrader(ctrl)
				mockUpgrader.EXPECT().UpgradeApplication(gomock.Any()).Return(errors.New("some error"))

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					versionGetter: mockVersionGetter,
					identity:      mockIdentity,
					store:         mockStore,
					prog:          mockProg,
					upgrader:      mockUpgrader,
				}
			},
			wantedErr: fmt.Errorf("upgrade application phonetool from version v0.0.0 to version %s: some error", deploy.LatestAppTemplateVersion),
		},
		"success": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockVersionGetter := mocks.NewMockversionGetter(ctrl)
				mockVersionGetter.EXPECT().Version().Return(deploy.LegacyAppTemplateVersion, nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				mockIdentity := mocks.NewMockidentityService(ctrl)
				mockIdentity.EXPECT().Get().Return(identity.Caller{Account: "1234"}, nil)

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{
					Name:               "phonetool",
					Domain:             "hello.com",
					DomainHostedZoneID: "2klfqok3",
				}, nil)

				mockUpgrader := mocks.NewMockappUpgrader(ctrl)
				mockUpgrader.EXPECT().UpgradeApplication(&deploy.CreateAppInput{
					Name:               "phonetool",
					AccountID:          "1234",
					DomainName:         "hello.com",
					DomainHostedZoneID: "2klfqok3",
					Version:            deploy.LatestAppTemplateVersion,
				}).Return(nil)

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					versionGetter: mockVersionGetter,
					identity:      mockIdentity,
					store:         mockStore,
					prog:          mockProg,
					upgrader:      mockUpgrader,
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := tc.given(ctrl)

			err := opts.Execute()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
