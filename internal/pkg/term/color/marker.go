// +build !windows

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package color

import "github.com/fatih/color"

// String markers to denote the status of an operation.
var (
	SuccessMarker = color.HiGreenString("✔")
	ErrorMarker   = color.HiRedString("✘")
)
