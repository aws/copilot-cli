// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockOverrider struct {
	out []byte
	err error
}

func (m *mockOverrider) Override(_ []byte) ([]byte, error) {
	return m.out, m.err
}

type mockOverriderWithDeps struct {
	*mockOverrider

	installCounter int
	installErr     error

	cleanUpCounter int
	cleanUp        func([]byte) ([]byte, error)
}

func (m *mockOverriderWithDeps) Install() error {
	m.installCounter += 1
	return m.installErr
}

func (m *mockOverriderWithDeps) CleanUp(in []byte) ([]byte, error) {
	m.cleanUpCounter += 1
	return m.cleanUp(in)
}

func TestTemplate(t *testing.T) {
	t.Parallel()
	t.Run("should return wrapped installation error", func(t *testing.T) {
		ovrdr := &mockOverriderWithDeps{
			installErr: errors.New(`run "npm install"`),
		}
		_, err := Bytes(nil, ovrdr)
		require.EqualError(t, err, `install dependencies before overriding: run "npm install"`)
	})
	t.Run("should return wrapped override error", func(t *testing.T) {
		ovrdr := &mockOverrider{
			err: errors.New(`run "cdk synth"`),
		}
		_, err := Bytes(nil, ovrdr)
		require.EqualError(t, err, `override document: run "cdk synth"`)
	})
	t.Run("should return wrapped clean up error", func(t *testing.T) {
		ovrdr := &mockOverriderWithDeps{
			mockOverrider: &mockOverrider{},
			cleanUp: func(_ []byte) ([]byte, error) {
				return nil, errors.New("unmarshal yaml")
			},
		}
		_, err := Bytes(nil, ovrdr)
		require.EqualError(t, err, "clean up overriden document: unmarshal yaml")
	})
	t.Run("should install, override, and then return cleaned up template", func(t *testing.T) {
		ovrdr := &mockOverriderWithDeps{
			mockOverrider: &mockOverrider{out: []byte("hello")},
			cleanUp: func(in []byte) ([]byte, error) {
				return append(in, '1'), nil
			},
		}
		out, err := Bytes(nil, ovrdr)
		require.NoError(t, err)
		require.Equal(t, []byte("hello1"), out)
	})
}
