// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type mockINI struct {
	sections []string
}

func (m *mockINI) Sections() []string {
	return m.sections
}

func TestConfig_Names(t *testing.T) {
	testCases := map[string]struct {
		ini *mockINI

		wantedNames []string
	}{
		"return nil if there are no sections in the file": {
			ini: &mockINI{
				sections: nil,
			},
		},
		"trim 'profile' from profile names": {
			ini: &mockINI{
				sections: []string{
					"profile profile1",
					"test",
					"default",
				},
			},

			wantedNames: []string{
				"profile1",
				"test",
				"default",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			conf := &Config{
				f: tc.ini,
			}

			// WHEN
			profiles := conf.Names()

			// THEN
			require.Equal(t, tc.wantedNames, profiles)
		})
	}
}
