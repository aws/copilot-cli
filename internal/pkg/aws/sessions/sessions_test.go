// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package sessions

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions/mocks"
	"github.com/golang/mock/gomock"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
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

func TestAreCredsFromEnvVars(t *testing.T) {
	testCases := map[string]struct {
		inSess *session.Session

		wantedOk  bool
		wantedErr error
	}{
		"returns true if the credentials come from environment variables": {
			inSess: &session.Session{
				Config: &aws.Config{
					Credentials: credentials.NewCredentials(mockProvider{
						value: credentials.Value{
							ProviderName: session.EnvProviderName,
						},
						err: nil,
					}),
				},
			},
			wantedOk: true,
		},
		"returns false if credentials are provided from a named profile": {
			inSess: &session.Session{
				Config: &aws.Config{
					Credentials: credentials.NewCredentials(mockProvider{
						value: credentials.Value{
							ProviderName: credentials.SharedCredsProviderName,
						},
						err: nil,
					}),
				},
			},
			wantedOk: false,
		},
		"returns a wrapped error if fails to fetch credentials": {
			inSess: &session.Session{
				Config: &aws.Config{
					Credentials: credentials.NewCredentials(mockProvider{
						value: credentials.Value{},
						err:   errors.New("some error"),
					}),
				},
			},
			wantedErr: errors.New("get credentials of session: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ok, err := AreCredsFromEnvVars(tc.inSess)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedOk, ok)
			}

		})
	}
}

func TestCreds(t *testing.T) {
	testCases := map[string]struct {
		inSess *session.Session

		wantedCreds credentials.Value
		wantedErr   error
	}{
		"returns values if provider is valid": {
			inSess: &session.Session{
				Config: &aws.Config{
					Credentials: credentials.NewCredentials(mockProvider{
						value: credentials.Value{
							AccessKeyID:     "abc",
							SecretAccessKey: "def",
						},
						err: nil,
					}),
				},
			},
			wantedCreds: credentials.Value{
				AccessKeyID:     "abc",
				SecretAccessKey: "def",
			},
		},
		"returns a wrapped error if fails to fetch credentials": {
			inSess: &session.Session{
				Config: &aws.Config{
					Credentials: credentials.NewCredentials(mockProvider{
						value: credentials.Value{},
						err:   errors.New("some error"),
					}),
				},
			},
			wantedErr: errors.New("get credentials of session: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			creds, err := Creds(tc.inSess)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedCreds, creds)
			}

		})
	}
}

func TestProvider_FromProfile(t *testing.T) {
	t.Run("error if region is missing", func(t *testing.T) {
		ogRegion := os.Getenv("AWS_REGION")
		ogDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
		defer func() {
			err := restoreEnvVar("AWS_REGION", ogRegion)
			require.NoError(t, err)

			err = restoreEnvVar("AWS_DEFAULT_REGION", ogDefaultRegion)
			require.NoError(t, err)
		}()

		// Since "walk-like-an-egyptian" is (very likely) a non-existent profile, whether the region information
		// is missing depends on whether the `AWS_REGION` environment variable is set.
		err := os.Unsetenv("AWS_REGION")
		require.NoError(t, err)
		err = os.Unsetenv("AWS_DEFAULT_REGION")
		require.NoError(t, err)

		// When
		sess, err := ImmutableProvider().FromProfile("walk-like-an-egyptian")

		// THEN
		require.NotNil(t, err)
		require.EqualError(t, errors.New("missing region configuration"), err.Error())
		require.Nil(t, sess)
	})

	t.Run("region information present", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		m := mocks.NewMocksessionValidator(ctrl)
		m.EXPECT().ValidateCredentials(gomock.Any()).Return(credentials.Value{}, nil)

		ogRegion := os.Getenv("AWS_REGION")
		defer func() {
			err := restoreEnvVar("AWS_REGION", ogRegion)
			require.NoError(t, err)
		}()

		// Since "walk-like-an-egyptian" is (very likely) a non-existent profile, whether the region information
		// is missing depends on whether the `AWS_REGION` environment variable is set.
		err := os.Setenv("AWS_REGION", "us-west-2")
		require.NoError(t, err)

		// WHEN
		provider := &Provider{
			sessionValidator: m,
		}

		sess, err := provider.FromProfile("walk-like-an-egyptian")

		// THEN
		require.NoError(t, err)
		require.Equal(t, "us-west-2", *sess.Config.Region)
	})

	t.Run("session credentials are incorrect", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		m := mocks.NewMocksessionValidator(ctrl)
		m.EXPECT().ValidateCredentials(gomock.Any()).Return(credentials.Value{}, context.DeadlineExceeded)

		ogRegion := os.Getenv("AWS_REGION")
		defer func() {
			err := restoreEnvVar("AWS_REGION", ogRegion)
			require.NoError(t, err)
		}()

		// Since "walk-like-an-egyptian" is (very likely) a non-existent profile, whether the region information
		// is missing depends on whether the `AWS_REGION` environment variable is set.
		err := os.Setenv("AWS_REGION", "us-west-2")
		require.NoError(t, err)

		// WHEN
		provider := &Provider{
			sessionValidator: m,
		}

		sess, err := provider.FromProfile("walk-like-an-egyptian")

		// THEN
		require.EqualError(t, err, "context deadline exceeded")
		require.Nil(t, sess)
	})
}

func restoreEnvVar(key string, originalValue string) error {
	if originalValue == "" {
		return os.Unsetenv(key)
	}
	return os.Setenv(key, originalValue)
}
