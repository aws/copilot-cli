// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import "github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/term/prompt"

type prompter interface {
	Get(message, help string, validator prompt.ValidatorFunc) (string, error)
	SelectOne(message, help string, options []string) (string, error)
	Confirm(message, help string, options ...prompt.ConfirmOption) (bool, error)
}
