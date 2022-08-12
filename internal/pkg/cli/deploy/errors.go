// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

type errSvcWithNoALBAliasDeployingToEnvWithImportedCerts struct {
	name    string
	envName string
}

func (e *errSvcWithNoALBAliasDeployingToEnvWithImportedCerts) Error() string {
	return fmt.Sprintf("cannot deploy service %s without http.alias to environment %s with certificate imported", e.name, e.envName)
}

type errSvcWithALBAliasHostedZoneWithCDNEnabled struct {
	envName string
}

func (e *errSvcWithALBAliasHostedZoneWithCDNEnabled) Error() string {
	return fmt.Sprintf("cannot specify alias hosted zones when cdn is enabled in environment %q", e.envName)
}

// RecommendActions returns recommended actions to be taken after the error.
// Implements main.actionRecommender interface.
func (e *errSvcWithALBAliasHostedZoneWithCDNEnabled) RecommendActions() string {
	msgs := []string{
		"If you already have an Load Balanced Web Service deployed, you can switch to CDN by running:",
		fmt.Sprintf("1. manually update the A-record value to be the CDN distribution domain name with %q set to %s.",
			"Evaluate target health", color.HighlightCode("True")),
		"2. Remove the alias hosted zones setting in the service manifest.",
		fmt.Sprintf("3. Reploy the service %s.", color.HighlightCode("copilot svc deploy")),
	}
	return strings.Join(msgs, "\n")
}
