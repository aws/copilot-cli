// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestFlag_portOverrides(t *testing.T) {
	tests := map[string]struct {
		in      []string
		want    portOverrides
		wantErr string
	}{
		"error: string": {
			in:      []string{"--p", "asdf"},
			wantErr: `invalid argument "asdf" for "--p" flag: should be in format 8080:80`,
		},
		"error: only one number": {
			in:      []string{"--p", "8080"},
			wantErr: `invalid argument "8080" for "--p" flag: should be in format 8080:80`,
		},
		"error: host not a number": {
			in:      []string{"--p", "asdf:8080"},
			wantErr: `invalid argument "asdf:8080" for "--p" flag: should be in format 8080:80`,
		},
		"error: container not a number": {
			in:      []string{"--p", "8080:asdf"},
			wantErr: `invalid argument "8080:asdf" for "--p" flag: should be in format 8080:80`,
		},
		"success: no port overrides": {},
		"success: one port override": {
			in: []string{"--p", "77:7777"},
			want: portOverrides{
				{
					host:      "77",
					container: "7777",
				},
			},
		},
		"success: multiple port override": {
			in: []string{"--p", "77:7777", "--p=9999:50"},
			want: portOverrides{
				{
					host:      "77",
					container: "7777",
				},
				{
					host:      "9999",
					container: "50",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var got portOverrides
			f := pflag.NewFlagSet("test", pflag.ContinueOnError)
			f.Var(&got, "p", "")

			err := f.Parse(tc.in)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
