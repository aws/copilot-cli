// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
)

// ErrInvalidAppManifestType occurs when a user requested a manifest template type that doesn't exist.
type ErrInvalidAppManifestType struct {
	Type string
}

func (e *ErrInvalidAppManifestType) Error() string {
	return fmt.Sprintf("invalid manifest type: %s", e.Type)
}
