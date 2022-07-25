//go:build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	spin "github.com/briandowns/spinner"
)

var charset = spin.CharSets[14]
