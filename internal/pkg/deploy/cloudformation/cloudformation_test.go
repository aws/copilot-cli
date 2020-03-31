// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/gobuffalo/packd"
)

const (
	mockTemplate = "mockTemplate"
)

func boxWithTemplateFile() packd.Box {
	box := packd.NewMemoryBox()

	box.AddString(stack.EnvTemplatePath, mockTemplate)

	return box
}
