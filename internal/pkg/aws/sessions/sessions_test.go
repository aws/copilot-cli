// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package sessions

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	value credentials.Value
	err error
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

func TestIsEnvVarProvider(t *testing.T) {
	testCases := map[string]struct {
		inSess *session.Session

		wantedOk bool
		wantedErr error
	} {
		"returns true if the credentials come from environment variables": {
			inSess: &session.Session{
				Config: &aws.Config{
					Credentials: credentials.NewCredentials(mockProvider{
						value: credentials.Value{
							ProviderName: credentials.EnvProviderName,
						},
						err:   nil,
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
						err:   nil,
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
			ok, err := IsEnvVarProvider(tc.inSess)

			if tc.wantedErr != nil {
				require.EqualError(t, tc.wantedErr, err.Error())
			} else {
				require.Equal(t, tc.wantedOk, ok)
			}

		})
	}
}
