// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the archer subcommands.
package cli

// actionCommand is the interface that every command that creates a resource implements.
type actionCommand interface {
	Ask() error
	Validate() error
	Execute() error
	RecommendedActions() []string
}
