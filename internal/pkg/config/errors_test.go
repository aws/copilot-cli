// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrNoSuchApplication(t *testing.T) {
	err := &ErrNoSuchApplication{ApplicationName: "chicken", AccountID: "12345", Region: "us-west-2"}
	require.EqualError(t, err, "couldn't find an application named chicken in account 12345 and region us-west-2")
}

func TestErrNoSuchApplication_Is(t *testing.T) {
	err := &ErrNoSuchApplication{ApplicationName: "chicken", AccountID: "12345", Region: "us-west-2"}
	testCases := map[string]struct {
		wantedSame bool
		otherError error
	}{
		"errors are same": {
			wantedSame: true,
			otherError: &ErrNoSuchApplication{ApplicationName: "chicken", AccountID: "12345", Region: "us-west-2"},
		},
		"errors have different values": {
			wantedSame: false,
			otherError: &ErrNoSuchApplication{ApplicationName: "rooster", AccountID: "12345", Region: "us-west-2"},
		},
		"errors are different type": {
			wantedSame: false,
			otherError: errors.New("different error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, err.Is(tc.otherError), tc.wantedSame)
		})
	}
}
func TestErrNoSuchEnvironment(t *testing.T) {
	err := &ErrNoSuchEnvironment{EnvironmentName: "test", ApplicationName: "chicken"}
	require.EqualError(t, err, "couldn't find environment test in the application chicken")
}

func TestErrNoSuchEnvironment_Is(t *testing.T) {
	err := &ErrNoSuchEnvironment{EnvironmentName: "test", ApplicationName: "chicken"}
	testCases := map[string]struct {
		wantedSame bool
		otherError error
	}{
		"errors are same": {
			wantedSame: true,
			otherError: &ErrNoSuchEnvironment{EnvironmentName: "test", ApplicationName: "chicken"},
		},
		"errors have different values": {
			wantedSame: false,
			otherError: &ErrNoSuchEnvironment{EnvironmentName: "test", ApplicationName: "rooster"},
		},
		"errors are different type": {
			wantedSame: false,
			otherError: errors.New("different error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, err.Is(tc.otherError), tc.wantedSame)
		})
	}
}

func TestErrNoSuchService(t *testing.T) {
	err := &ErrNoSuchService{Name: "api", App: "chicken"}
	require.EqualError(t, err, "couldn't find service api in the application chicken")
}

func TestErrNoSuchService_Is(t *testing.T) {
	err := &ErrNoSuchService{Name: "api", App: "chicken"}
	testCases := map[string]struct {
		wantedSame bool
		otherError error
	}{
		"errors are same": {
			wantedSame: true,
			otherError: &ErrNoSuchService{Name: "api", App: "chicken"},
		},
		"errors have different values": {
			wantedSame: false,
			otherError: &ErrNoSuchService{Name: "api", App: "rooster"},
		},
		"errors are different type": {
			wantedSame: false,
			otherError: errors.New("different error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, err.Is(tc.otherError), tc.wantedSame)
		})
	}
}

func TestErrNoSuchJob(t *testing.T) {
	err := &ErrNoSuchJob{Name: "mailer", App: "cool"}
	require.EqualError(t, err, "couldn't find job mailer in the application cool")
}

func TestErrNoSuchJob_Is(t *testing.T) {
	err := &ErrNoSuchJob{Name: "mailer", App: "cool"}
	testCases := map[string]struct {
		wantedSame bool
		otherError error
	}{
		"errors are same": {
			wantedSame: true,
			otherError: &ErrNoSuchJob{Name: "mailer", App: "cool"},
		},
		"errors have different values": {
			wantedSame: false,
			otherError: &ErrNoSuchJob{Name: "ranker", App: "cool"},
		},
		"errors are different types": {
			wantedSame: false,
			otherError: errors.New("something else broke"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, err.Is(tc.otherError), tc.wantedSame)
		})
	}
}
