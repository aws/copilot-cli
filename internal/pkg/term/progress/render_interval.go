// +build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import "time"

const (
	renderInterval = 100 * time.Millisecond // How frequently Render should be invoked.
)
