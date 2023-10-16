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
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type versionGetterDouble struct {
	VersionFn func() (string, error)
}

func (d *versionGetterDouble) Version() (string, error) {
	return d.VersionFn()
}

type appUpgradeMocks struct {
	storeSvc *mocks.Mockstore
	sel      *mocks.MockappSelector
}

func TestAppUpgradeOpts_Validate(t *testing.T) {
	testError := errors.New("some error")
	testCases := map[string]struct {
		inAppName  string
		setupMocks func(mocks appUpgradeMocks)

		wantedError error
	}{
		"valid app name": {
			inAppName: "my-app",

			setupMocks: func(m appUpgradeMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
			},
			wantedError: nil,
		},
		"invalid app name": {
			inAppName: "my-app",

			setupMocks: func(m appUpgradeMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("get application %s: %w", "my-app", testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)

			mocks := appUpgradeMocks{
				storeSvc: mockStoreReader,
			}
			tc.setupMocks(mocks)

			opts := &appUpgradeOpts{
				appUpgradeVars: appUpgradeVars{
					name: tc.inAppName,
				},
				store: mockStoreReader,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAppUpgradeOpts_Ask(t *testing.T) {
	testError := errors.New("some error")
	testCases := map[string]struct {
		inApp string

		setupMocks func(mocks appUpgradeMocks)

		wantedApp   string
		wantedError error
	}{
		"with all flags": {
			inApp: "my-app",

			setupMocks: func(m appUpgradeMocks) {},

			wantedApp:   "my-app",
			wantedError: nil,
		},
		"prompt for all input": {
			inApp: "",

			setupMocks: func(m appUpgradeMocks) {
				m.sel.EXPECT().Application(appUpgradeNamePrompt, appUpgradeNameHelpPrompt).Return("my-app", nil)
			},
			wantedApp:   "my-app",
			wantedError: nil,
		},
		"returns error if failed to select application": {
			inApp: "",

			setupMocks: func(m appUpgradeMocks) {
				m.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Return("", testError)
			},

			wantedError: fmt.Errorf("select application: %w", testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := appUpgradeMocks{
				sel: mocks.NewMockappSelector(ctrl),
			}
			tc.setupMocks(mocks)

			opts := &appUpgradeOpts{
				appUpgradeVars: appUpgradeVars{
					name: tc.inApp,
				},
				sel: mocks.sel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, opts.name, "expected app names to match")

			}
		})
	}
}

func TestAppUpgradeOpts_Execute(t *testing.T) {
	const mockTemplateVersion = "v1.29.0"
	versionGetterLegacy := func(string) (versionGetter, error) {
		return &versionGetterDouble{
			VersionFn: func() (string, error) {
				return version.LegacyAppTemplate, nil
			},
		}, nil
	}

	testCases := map[string]struct {
		given     func(ctrl *gomock.Controller) *appUpgradeOpts
		wantedErr error
	}{
		"should return error if fail to get template version": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					newVersionGetter: func(string) (versionGetter, error) {
						return &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", errors.New("some error")
							},
						}, nil
					},
				}
			},
			wantedErr: fmt.Errorf("get template version of application phonetool: some error"),
		},
		"should return if app is up-to-date": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					newVersionGetter: func(string) (versionGetter, error) {
						return &versionGetterDouble{
							VersionFn: func() (string, error) {
								return mockTemplateVersion, nil
							},
						}, nil
					},
				}
			},
		},
		"should return error if fail to get application": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					newVersionGetter: versionGetterLegacy,
					store:            mockStore,
				}
			},
			wantedErr: fmt.Errorf("get application phonetool: some error"),
		},
		"should return error if fail to get identity": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockIdentity := mocks.NewMockidentityService(ctrl)
				mockIdentity.EXPECT().Get().Return(identity.Caller{}, errors.New("some error"))

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					newVersionGetter: versionGetterLegacy,
					identity:         mockIdentity,
					store:            mockStore,
				}
			},
			wantedErr: fmt.Errorf("get identity: some error"),
		},
		"should return error if fail to get hostedzone id": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockIdentity := mocks.NewMockidentityService(ctrl)
				mockIdentity.EXPECT().Get().Return(identity.Caller{Account: "1234"}, nil)

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{
					Name:   "phonetool",
					Domain: "foobar.com",
				}, nil)

				mockRoute53 := mocks.NewMockdomainHostedZoneGetter(ctrl)
				mockRoute53.EXPECT().PublicDomainHostedZoneID("foobar.com").Return("", errors.New("some error"))

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					newVersionGetter: versionGetterLegacy,
					identity:         mockIdentity,
					store:            mockStore,
					route53:          mockRoute53,
				}
			},
			wantedErr: fmt.Errorf("get hosted zone ID for domain foobar.com: some error"),
		},
		"should return error if fail to upgrade application": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockIdentity := mocks.NewMockidentityService(ctrl)
				mockIdentity.EXPECT().Get().Return(identity.Caller{Account: "1234"}, nil)

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				mockStore.EXPECT().UpdateApplication(&config.Application{Name: "phonetool"}).Return(nil)

				mockUpgrader := mocks.NewMockappUpgrader(ctrl)
				mockUpgrader.EXPECT().UpgradeApplication(gomock.Any()).Return(errors.New("some error"))

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					newVersionGetter: versionGetterLegacy,
					identity:         mockIdentity,
					store:            mockStore,
					upgrader:         mockUpgrader,
				}
			},
			wantedErr: fmt.Errorf("upgrade application phonetool from version v0.0.0 to version %s: some error", mockTemplateVersion),
		},
		"success": {
			given: func(ctrl *gomock.Controller) *appUpgradeOpts {
				mockIdentity := mocks.NewMockidentityService(ctrl)
				mockIdentity.EXPECT().Get().Return(identity.Caller{Account: "1234"}, nil)

				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{
					Name:   "phonetool",
					Domain: "hello.com",
				}, nil)
				mockStore.EXPECT().UpdateApplication(&config.Application{
					Name:               "phonetool",
					Domain:             "hello.com",
					DomainHostedZoneID: "2klfqok3",
				}).Return(nil)

				mockRoute53 := mocks.NewMockdomainHostedZoneGetter(ctrl)
				mockRoute53.EXPECT().PublicDomainHostedZoneID("hello.com").Return("2klfqok3", nil)

				mockUpgrader := mocks.NewMockappUpgrader(ctrl)
				mockUpgrader.EXPECT().UpgradeApplication(&deploy.CreateAppInput{
					Name:               "phonetool",
					AccountID:          "1234",
					DomainName:         "hello.com",
					DomainHostedZoneID: "2klfqok3",
					Version:            mockTemplateVersion,
				}).Return(nil)

				return &appUpgradeOpts{
					appUpgradeVars: appUpgradeVars{
						name: "phonetool",
					},
					newVersionGetter: versionGetterLegacy,
					identity:         mockIdentity,
					store:            mockStore,
					upgrader:         mockUpgrader,
					route53:          mockRoute53,
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := tc.given(ctrl)
			opts.templateVersion = mockTemplateVersion

			err := opts.Execute()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
