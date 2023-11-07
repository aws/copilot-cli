//go:build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package file

import "path/filepath"

// IsHiddenFile returns true if the file is hidden on non-windows. The filename must be non-empty.
func IsHiddenFile(filename string) (bool, error) {
	return filepath.Base(filename)[0] == '.', nil
}
