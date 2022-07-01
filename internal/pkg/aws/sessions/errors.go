// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package sessions

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

type errMissingRegion struct{}

// Implements error interface.
func (e *errMissingRegion) Error() string {
	return "missing region configuration"
}

// RecommendActions returns recommended actions to be taken after the error.
// Implements main.actionRecommender interface.
func (e *errMissingRegion) RecommendActions() string { // implements new actionRecommender interface.
	return fmt.Sprintf(`It looks like your AWS region configuration is missing.
- We recommend including your region configuration in the "~/.aws/config" file.
- Alternatively, you can run %s to set the environment variable.
More information: https://aws.github.io/copilot-cli/docs/credentials/`, color.HighlightCode("export AWS_REGION=<application region>"))
}

type errCredRetrieval struct {
	profile   string
	parentErr error
}

// Implements error interface.
func (e *errCredRetrieval) Error() string {
	return e.parentErr.Error()
}

// RecommendActions returns recommended actions to be taken after the error.
// Implements main.actionRecommender interface.
func (e *errCredRetrieval) RecommendActions() string {
	notice := "It looks like your credential settings are misconfigured or missing"
	if e.profile != "" {
		notice = fmt.Sprintf("It looks like your profile [%s] is misconfigured or missing", e.profile)
	}
	return fmt.Sprintf(`%s:
https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials
- We recommend including your credentials in the shared credentials file.
- Alternatively, you can also set credentials through 
	* Environment Variables
	* EC2 Instance Metadata (credentials only)
More information: https://aws.github.io/copilot-cli/docs/credentials/`, notice)
}

func isCredRetrievalErr(err error) bool {
	return strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "NoCredentialProviders")
}
