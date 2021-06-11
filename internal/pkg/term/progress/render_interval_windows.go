// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"os"
	"time"
)

var (
	// Windows flickers too frequently if the interval is too short.
	renderInterval = 500 * time.Millisecond // How frequently Render should be invoked.
)

func init() {
	if os.Getenv("CI") == "true" {
		renderInterval = 30 * time.Second
	}
}
