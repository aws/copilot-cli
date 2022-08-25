// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/aws/copilot-cli/internal/pkg/aws/ssm"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type secretInitMocks struct {
	mockFS    afero.Fs
	mockStore *mocks.Mockstore
}

func TestSecretInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inApp           string
		inName          string
		inValues        map[string]string
		inOverwrite     bool
		inInputFilePath string

		setupMocks func(m secretInitMocks)

		wantedError error
	}{
		"skip validation if app flag is not set": {
			setupMocks: func(m secretInitMocks) {},
		},
		"valid with input file": {
			inInputFilePath: "./deep/secrets.yml",
			inOverwrite:     true,

			setupMocks: func(m secretInitMocks) {
				m.mockFS.MkdirAll("deep", 0755)
				afero.WriteFile(m.mockFS, "deep/secrets.yml", []byte("FROM nginx"), 0644)
			},
		},
		"valid with name and value": {
			inName: "where_is_the_dragon",
			inValues: map[string]string{
				"good_village": "on_top_of_the_mountain",
				"bad_village":  "by_the_volcano",
			},
			inApp:       "dragon_slaying",
			inOverwrite: true,

			setupMocks: func(m secretInitMocks) {
				m.mockStore.EXPECT().GetApplication("dragon_slaying").Return(&config.Application{}, nil)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "good_village").Return(&config.Environment{}, nil)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "bad_village").Return(&config.Environment{}, nil)
			},
		},
		"error getting app": {
			inApp: "dragon_befriending",
			setupMocks: func(m secretInitMocks) {
				m.mockStore.EXPECT().GetApplication("dragon_befriending").Return(&config.Application{}, errors.New("some error"))
			},
			wantedError: errors.New("get application dragon_befriending: some error"),
		},
		"error getting env from the app": {
			inName: "where_is_the_dragon",
			inValues: map[string]string{
				"good_village":    "on_top_of_the_mountain",
				"bad_village":     "by_the_volcano",
				"neutral_village": "there_is_no_such_village",
			},
			inApp: "dragon_slaying",
			setupMocks: func(m secretInitMocks) {
				m.mockStore.EXPECT().GetApplication("dragon_slaying").Return(&config.Application{}, nil)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "good_village").Return(&config.Environment{}, nil).MinTimes(0).MaxTimes(1)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "bad_village").Return(&config.Environment{}, nil).MinTimes(0).MaxTimes(1)
				m.mockStore.EXPECT().GetEnvironment("dragon_slaying", "neutral_village").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get environment neutral_village in application dragon_slaying: some error"),
		},
		"invalid input file name": {
			inInputFilePath: "weird/path/to/secrets",
			setupMocks:      func(m secretInitMocks) {},
			wantedError:     fmt.Errorf("open %s: file does not exist", filepath.FromSlash("weird/path/to/secrets")),
		},
		"error if input file name is specified with name": {
			inName:          "db-password",
			inInputFilePath: "path/to/file",
			setupMocks:      func(m secretInitMocks) {},
			wantedError:     errors.New("cannot specify `--cli-input-yaml` with `--name`"),
		},
		"error if input file name is specified with values": {
			inValues: map[string]string{
				"test": "test-db",
				"prod": "prod-db",
			},
			inInputFilePath: "path/to/file",
			setupMocks:      func(m secretInitMocks) {},
			wantedError:     errors.New("cannot specify `--cli-input-yaml` with `--values`"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)

			opts := secretInitOpts{
				secretInitVars: secretInitVars{
					appName:       tc.inApp,
					name:          tc.inName,
					values:        tc.inValues,
					inputFilePath: tc.inInputFilePath,
					overwrite:     tc.inOverwrite,
				},
				fs:    &afero.Afero{Fs: afero.NewMemMapFs()},
				store: mockStore,
			}

			m := secretInitMocks{
				mockFS:    opts.fs,
				mockStore: mockStore,
			}
			tc.setupMocks(m)

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

type secretInitAskMocks struct {
	mockStore    *mocks.Mockstore
	mockPrompter *mocks.Mockprompter
	mockSelector *mocks.MockappSelector
}

func TestSecretInitOpts_Ask(t *testing.T) {
	var (
		wantedName   = "db-password"
		wantedApp    = "my-app"
		wantedValues = map[string]string{
			"test": "test-password",
			"dev":  "dev-password",
			"prod": "prod-password",
		}
		wantedVars = secretInitVars{
			appName: wantedApp,
			name:    wantedName,
			values:  wantedValues,
		}
	)
	testCases := map[string]struct {
		inAppName string
		inName    string
		inValues  map[string]string

		setupMocks func(m secretInitAskMocks)

		wantedVars  secretInitVars
		wantedError error
	}{
		"prompt to select an app if not specified": {
			inName:   wantedName,
			inValues: wantedValues,
			setupMocks: func(m secretInitAskMocks) {
				m.mockSelector.EXPECT().Application(secretInitAppPrompt, gomock.Any()).Return(wantedApp, nil)
			},
			wantedVars: wantedVars,
		},
		"error prompting to select an app": {
			setupMocks: func(m secretInitAskMocks) {
				m.mockSelector.EXPECT().Application(secretInitAppPrompt, gomock.Any()).Return("", errors.New("some error"))
			},
			wantedError: errors.New("ask for an application to add the secret to: some error"),
		},
		"do not prompt for app if specified": {
			inAppName: wantedApp,
			inName:    wantedName,
			inValues:  wantedValues,
			setupMocks: func(m secretInitAskMocks) {
				m.mockSelector.EXPECT().Application(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedVars: secretInitVars{
				appName: wantedApp,
				name:    wantedName,
				values:  wantedValues,
			},
		},
		"ask for a secret name if not specified": {
			inAppName: wantedApp,
			inValues:  wantedValues,
			setupMocks: func(m secretInitAskMocks) {
				m.mockPrompter.EXPECT().Get(secretInitSecretNamePrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return("db-password", nil)
			},
			wantedVars: wantedVars,
		},
		"error prompting for a secret name": {
			inAppName: wantedApp,
			inValues:  wantedValues,
			setupMocks: func(m secretInitAskMocks) {
				m.mockPrompter.EXPECT().Get(secretInitSecretNamePrompt, gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedError: errors.New("ask for the secret name: some error"),
		},
		"do not ask for a secret name if specified": {
			inName:    "db-password",
			inAppName: wantedApp,
			inValues:  wantedValues,
			setupMocks: func(m secretInitAskMocks) {
				m.mockPrompter.EXPECT().Get(secretInitSecretNamePrompt, gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedVars: wantedVars,
		},
		"ask for values for each existing environment if not specified": {
			inAppName: wantedApp,
			inName:    wantedName,
			setupMocks: func(m secretInitAskMocks) {
				m.mockStore.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name: "test",
					},
					{
						Name: "dev",
					},
					{
						Name: "prod",
					},
				}, nil)
				m.mockPrompter.EXPECT().GetSecret(fmt.Sprintf(fmtSecretInitSecretValuePrompt, "db-password", "test"), gomock.Any(), gomock.Any()).
					Return("test-password", nil)
				m.mockPrompter.EXPECT().GetSecret(fmt.Sprintf(fmtSecretInitSecretValuePrompt, "db-password", "dev"), gomock.Any(), gomock.Any()).
					Return("dev-password", nil)
				m.mockPrompter.EXPECT().GetSecret(fmt.Sprintf(fmtSecretInitSecretValuePrompt, "db-password", "prod"), gomock.Any(), gomock.Any()).
					Return("prod-password", nil)
			},
			wantedVars: wantedVars,
		},
		"error listing environments": {
			inAppName: wantedApp,
			inName:    wantedName,
			setupMocks: func(m secretInitAskMocks) {
				m.mockStore.EXPECT().ListEnvironments("my-app").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("list environments in app my-app: some error"),
		},
		"error prompting for values": {
			inAppName: wantedApp,
			inName:    wantedName,
			setupMocks: func(m secretInitAskMocks) {
				m.mockStore.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name: "test",
					},
					{
						Name: "dev",
					},
					{
						Name: "prod",
					},
				}, nil)
				m.mockPrompter.EXPECT().GetSecret(fmt.Sprintf(fmtSecretInitSecretValuePrompt, "db-password", "test"), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
				m.mockPrompter.EXPECT().GetSecret(fmt.Sprintf(fmtSecretInitSecretValuePrompt, "db-password", "dev"), gomock.Any(), gomock.Any()).MinTimes(0).MaxTimes(1)
				m.mockPrompter.EXPECT().GetSecret(fmt.Sprintf(fmtSecretInitSecretValuePrompt, "db-password", "prod"), gomock.Any(), gomock.Any()).MinTimes(0).MaxTimes(1)
			},
			wantedError: errors.New("get secret value for db-password in environment test: some error"),
		},
		"error if no env is found": {
			inAppName: wantedApp,
			inName:    wantedName,
			setupMocks: func(m secretInitAskMocks) {
				m.mockStore.EXPECT().ListEnvironments(wantedApp).Return([]*config.Environment{}, nil)
			},
			wantedError: errors.New("no environment is found in app my-app"),
		},
		"do not ask for values if specified": {
			inAppName:  wantedApp,
			inName:     wantedName,
			inValues:   wantedValues,
			setupMocks: func(m secretInitAskMocks) {},
			wantedVars: wantedVars,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := secretInitAskMocks{
				mockPrompter: mocks.NewMockprompter(ctrl),
				mockSelector: mocks.NewMockappSelector(ctrl),
				mockStore:    mocks.NewMockstore(ctrl),
			}

			opts := secretInitOpts{
				secretInitVars: secretInitVars{
					appName: tc.inAppName,
					name:    tc.inName,
					values:  tc.inValues,
				},
				prompter: m.mockPrompter,
				store:    m.mockStore,
				selector: m.mockSelector,
			}

			tc.setupMocks(m)

			err := opts.Ask()
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedVars, opts.secretInitVars)
			} else {
				require.EqualError(t, tc.wantedError, err.Error())
			}
		})
	}
}

type secretInitExecuteMocks struct {
	mockStore                   *mocks.Mockstore
	mockSecretPutter            *mocks.MocksecretPutter
	mockEnvCompatibilityChecker *mocks.MockversionCompatibilityChecker
}

func TestSecretInitOpts_Execute(t *testing.T) {
	var (
		testApp    = "test-app"
		testName   = "db-password"
		testValues = map[string]string{
			"test": "test-password",
			"prod": "prod-password",
		}
	)
	testCases := map[string]struct {
		inAppName string

		inName   string
		inValues map[string]string

		inInputFilePath string

		inOverwrite bool

		mockInputFileContent []byte
		setupMocks           func(m secretInitExecuteMocks)

		wantedError error
	}{
		"successfully create secrets in two environments": {
			inAppName: testApp,
			inName:    testName,
			inValues:  testValues,

			setupMocks: func(m secretInitExecuteMocks) {
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/test/secrets/db-password",
					Value:     "test-password",
					Overwrite: false,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "test",
					},
				}).Return(&ssm.PutSecretOutput{
					Version: aws.Int64(1),
				}, nil)
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/prod/secrets/db-password",
					Value:     "prod-password",
					Overwrite: false,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "prod",
					},
				}).Return(&ssm.PutSecretOutput{
					Version: aws.Int64(1),
				}, nil)
				m.mockEnvCompatibilityChecker.EXPECT().Version().Return("v1.10.0", nil).Times(2)
			},
		},
		"should make calls to overwrite if overwrite is specified": {
			inAppName:   testApp,
			inName:      testName,
			inValues:    testValues,
			inOverwrite: true,

			setupMocks: func(m secretInitExecuteMocks) {
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/test/secrets/db-password",
					Value:     "test-password",
					Overwrite: true,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "test",
					},
				}).Return(&ssm.PutSecretOutput{
					Version: aws.Int64(1),
				}, nil)
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/prod/secrets/db-password",
					Value:     "prod-password",
					Overwrite: true,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "prod",
					},
				}).Return(&ssm.PutSecretOutput{
					Version: aws.Int64(1),
				}, nil)
				m.mockEnvCompatibilityChecker.EXPECT().Version().Return("v1.10.0", nil).Times(2)
			},
		},
		"do not throw error if parameter already exists": {
			inAppName: testApp,
			inName:    testName,
			inValues:  testValues,

			setupMocks: func(m secretInitExecuteMocks) {
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/test/secrets/db-password",
					Value:     "test-password",
					Overwrite: false,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "test",
					},
				}).Return(nil, &ssm.ErrParameterAlreadyExists{})
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/prod/secrets/db-password",
					Value:     "prod-password",
					Overwrite: false,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "prod",
					},
				}).Return(&ssm.PutSecretOutput{
					Version: aws.Int64(1),
				}, nil)
				m.mockEnvCompatibilityChecker.EXPECT().Version().Return("v1.10.0", nil).Times(2)
			},
		},
		"a secret fails to create in some environments": {
			inAppName: testApp,
			inName:    testName,
			inValues:  testValues,

			setupMocks: func(m secretInitExecuteMocks) {
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/test/secrets/db-password",
					Value:     "test-password",
					Overwrite: false,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "test",
					},
				}).Return(nil, errors.New("some error"))
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/prod/secrets/db-password",
					Value:     "prod-password",
					Overwrite: false,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "prod",
					},
				}).Return(&ssm.PutSecretOutput{
					Version: aws.Int64(1),
				}, nil)
				m.mockEnvCompatibilityChecker.EXPECT().Version().Return("v1.10.0", nil).Times(2)
			},

			wantedError: &errSecretFailedInSomeEnvironments{
				secretName: testName,
				errorsForEnvironments: map[string]error{
					"test": errors.New("some error"),
				},
			},
		},
		"some secrets fail to create during a batch operation": {
			inAppName:       testApp,
			inInputFilePath: "some/file",

			mockInputFileContent: []byte(`db-password:
    test: test-password
    prod: prod-password
db-host:
    test: test-host`),
			setupMocks: func(m secretInitExecuteMocks) {
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/test/secrets/db-password",
					Value:     "test-password",
					Overwrite: false,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "test",
					},
				}).Return(nil, errors.New("some error for db-password in test"))
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/prod/secrets/db-password",
					Value:     "prod-password",
					Overwrite: false,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "prod",
					},
				}).Return(&ssm.PutSecretOutput{
					Version: aws.Int64(1),
				}, nil)
				m.mockSecretPutter.EXPECT().PutSecret(ssm.PutSecretInput{
					Name:      "/copilot/test-app/test/secrets/db-host",
					Value:     "test-host",
					Overwrite: false,
					Tags: map[string]string{
						deploy.AppTagKey: "test-app",
						deploy.EnvTagKey: "test",
					},
				}).Return(nil, errors.New("some error for db-host in test"))
				m.mockEnvCompatibilityChecker.EXPECT().Version().Return("v1.10.0", nil).Times(2)
			},

			wantedError: &errBatchPutSecretsFailed{
				errors: []*errSecretFailedInSomeEnvironments{
					{
						secretName: "db-password",
						errorsForEnvironments: map[string]error{
							"test": errors.New("some error for db-password in test"),
						},
					},
					{
						secretName: "db-host",
						errorsForEnvironments: map[string]error{
							"test": errors.New("some error for db-host in test"),
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := secretInitExecuteMocks{
				mockStore:                   mocks.NewMockstore(ctrl),
				mockSecretPutter:            mocks.NewMocksecretPutter(ctrl),
				mockEnvCompatibilityChecker: mocks.NewMockversionCompatibilityChecker(ctrl),
			}
			tc.setupMocks(m)

			opts := secretInitOpts{
				secretInitVars: secretInitVars{
					appName:       tc.inAppName,
					name:          tc.inName,
					values:        tc.inValues,
					overwrite:     tc.inOverwrite,
					inputFilePath: tc.inInputFilePath,
				},
				store: m.mockStore,

				secretPutters:           make(map[string]secretPutter),
				envCompatibilityChecker: make(map[string]versionCompatibilityChecker),
				readFile: func() ([]byte, error) {
					return tc.mockInputFileContent, nil
				},
			}

			opts.configureClientsForEnv = func(envName string) error {
				opts.secretPutters[envName] = m.mockSecretPutter
				opts.envCompatibilityChecker[envName] = m.mockEnvCompatibilityChecker
				return nil
			}

			err := opts.Execute()
			if tc.wantedError == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func Test_SecretInitParseSecretsInputFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		opts := secretInitOpts{
			readFile: func() ([]byte, error) {
				raw := `db-password:
    test: test-password
    prod: prod-password
db-host:
    test: test-host
    prod: prod-host
test-only-secret:
    test: test-only`
				return []byte(raw), nil
			},
		}

		expected := map[string]map[string]string{
			"db-password": {
				"test": "test-password",
				"prod": "prod-password",
			},
			"db-host": {
				"test": "test-host",
				"prod": "prod-host",
			},
			"test-only-secret": {
				"test": "test-only",
			},
		}

		secrets, err := opts.parseSecretsInputFile()
		require.NoError(t, err)
		require.Equal(t, expected, secrets)
	})
}
