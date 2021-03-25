// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
)

type cfn interface {
	Describe(name string) (*cloudformation.StackDescription, error)
	StackResources(name string) ([]*cloudformation.StackResource, error)
	Metadata(name string) (string, error)
}
