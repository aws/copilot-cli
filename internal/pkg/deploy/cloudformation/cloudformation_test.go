// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/gobuffalo/packd"
)

const (
	mockTemplate = "mockTemplate"
)

func boxWithTemplateFile() packd.Box {
	box := packd.NewMemoryBox()

	box.AddString(template.EnvCFTemplatePath, mockTemplate)

	return box
}
