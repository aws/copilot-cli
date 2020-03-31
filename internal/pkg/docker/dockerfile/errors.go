// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package dockerfile

import (
	"fmt"
)

// ErrExposeNoMatch indicates that no provided ports were parseable
type ErrExposeNoMatch struct {
	Line string
}

func (e *ErrExposeNoMatch) Error() string {
	return fmt.Sprintf("couldn't interpret port from statement %s",
		e.Line)
}

// ErrInvalidPort means that while there was a port provided, it was out of bounds
type ErrInvalidPort struct {
	Match string
}

func (e *ErrInvalidPort) Error() string {
	return fmt.Sprintf("port %s is invalid", e.Match)
}

// ErrNoExpose means there were no documented EXPOSE statements in the given dockerfile.
type ErrNoExpose struct {
	Dockerfile string
}

func (e *ErrNoExpose) Error() string {
	return fmt.Sprintf("no EXPOSE statements in Dockerfile %s", e.Dockerfile)
}

// ErrMultiplePorts indicates that more than one port was provided by EXPOSE statements
type ErrMultiplePorts struct {
	Dockerfile string
	N          int
}

func (e *ErrMultiplePorts) Error() string {
	return fmt.Sprintf("$d ports exposed in Dockerfile %s", e.N, e.Dockerfile)
}
