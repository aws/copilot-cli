// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import "fmt"

// ErrDirNotExist occurs when an addons directory for an application does not exist.
type ErrDirNotExist struct {
	AppName   string
	ParentErr error
}

func (e *ErrDirNotExist) Error() string {
	return fmt.Sprintf("read addons directory for application %s: %v", e.AppName, e.ParentErr)
}
