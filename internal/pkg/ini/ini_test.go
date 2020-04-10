// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ini

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
)

func TestINI_Sections(t *testing.T) {
	// GIVEN
	content := `[paths]
data = /home/git/grafana

[server]
protocol = http

`
	cfg, _ := ini.Load([]byte(content))
	ini := &INI{cfg: cfg}

	// WHEN
	actualNames := ini.Sections()

	// THEN
	require.Equal(t, []string{"paths", "server"}, actualNames)
}
