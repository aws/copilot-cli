// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package sessions

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// ErrMissingRegion is returned when the region information is missing from AWS configuration.
//var ErrMissingRegion = errors.New("missing region configuration")

type ErrMissingRegion struct{}

func (e *ErrMissingRegion) Error() string {
	return "missing region configuration"
} // implements the error interface.

func (e *ErrMissingRegion) RecommendActions() string { // implements new actionRecommender interface.
	return fmt.Sprintf(`It looks like your AWS region configuration is missing.
- We recommend including your region configuration in the "~/.aws/config" file.
- Alternatively, you can run %s to set the environment variable.
More information: https://aws.github.io/copilot-cli/docs/credentials/
`, color.HighlightCode("export AWS_REGION=<application region>"))
}
