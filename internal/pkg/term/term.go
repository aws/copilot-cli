// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package term provides functionality to interact with terminals.
package term

// Progress is the interface to inform the user that a long operation is taking place.
type Progress interface {
	// Start starts displaying progress with a label.
	Start(label string)
	// Stop ends displaying progress with a label.
	Stop(label string)
	// Tips writes additional information in between the start and stop stages.
	Tips([]string)
}
