// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"errors"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestDecomposeService(t *testing.T) {
	testCases := map[string]struct {
		filename string
		svcName  string

		wantSvc     manifest.BackendServiceConfig
		wantIgnored IgnoredKeys
		wantError   error
	}{
		"no services": {
			filename: "empty-compose.yml",
			svcName:  "test",

			wantError: errors.New("compose file has no services"),
		},
		"bad services": {
			filename: "bad-services-compose.yml",
			svcName:  "test",

			wantError: errors.New("\"services\" top-level element was not a map, was: invalid"),
		},
		"wrong name": {
			filename: "unsupported-keys.yml",
			svcName:  "test",

			wantError: errors.New("no service named \"test\" in this Compose file, valid services are: [fatal1 fatal2 fatal3]"),
		},
		"invalid service not a map": {
			filename: "invalid-compose.yml",
			svcName:  "invalid2",

			wantError: errors.New("\"services.invalid2\" element was not a map"),
		},
		"unsupported keys fatal1": {
			filename: "unsupported-keys.yml",
			svcName:  "fatal1",

			wantError: errors.New("\"services.fatal1\" relies on fatally-unsupported Compose keys: [external_links privileged]"),
		},
		"unsupported keys fatal2": {
			filename: "unsupported-keys.yml",
			svcName:  "fatal2",

			wantError: errors.New("convert Compose service to Copilot manifest: convert image config: `build.ssh` and `build.secrets` are not supported yet, see https://github.com/aws/copilot-cli/issues/2090 for details"),
		},
		"unsupported keys fatal3": {
			filename: "unsupported-keys.yml",
			svcName:  "fatal3",

			wantError: errors.New("\"services.fatal3\" relies on fatally-unsupported Compose keys: [domainname init]"),
		},
		"invalid compose": {
			filename: "invalid-compose.yml",
			svcName:  "invalid",

			wantError: errors.New("load Compose project: services.invalid.build.ssh must be a mapping"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", tc.filename)
			cfg, err := os.ReadFile(path)
			require.NoError(t, err)

			svc, ign, err := decomposeService(cfg, tc.svcName)

			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantSvc, svc)
				require.Equal(t, tc.wantIgnored, ign)
			}
		})
	}
}
