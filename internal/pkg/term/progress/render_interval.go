//go:build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"os"
	"time"
)

var (
	renderInterval = 100 * time.Millisecond // How frequently Render should be invoked.
)

func init() {
	// The CI environment variable is set to "true" by default by GitHub actions and GitLab.
	if os.Getenv("CI") == "true" {
		renderInterval = 30 * time.Second
	}
}
