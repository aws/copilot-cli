// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerfile

import "fmt"

// ErrInvalidPort means that while there was a port provided, it was out of bounds or unparseable
type ErrInvalidPort struct {
	Match string
}

func (e ErrInvalidPort) Error() string {
	return fmt.Sprintf("parse EXPOSE: port represented at %s is invalid or unparseable", e.Match)
}

// ErrNoExpose means there were no documented EXPOSE statements in the given dockerfile.
type ErrNoExpose struct {
	Dockerfile string
}

func (e ErrNoExpose) Error() string {
	return fmt.Sprintf("parse EXPOSE: no EXPOSE statements in Dockerfile %s", e.Dockerfile)
}
