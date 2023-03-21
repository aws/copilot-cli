// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/dustin/go-humanize/english"
)

type errSvcWithNoALBAliasDeployingToEnvWithImportedCerts struct {
	name    string
	envName string
}

func (e *errSvcWithNoALBAliasDeployingToEnvWithImportedCerts) Error() string {
	return fmt.Sprintf(`cannot deploy service %s without "alias" to environment %s with certificate imported`, e.name, e.envName)
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
		"If you already have a Load Balanced Web Service deployed, you can switch to CDN by:",
		" 1. Updating the A-record value to be the CDN distribution domain name.",
		fmt.Sprintf(" 2. Removing the %s setting from the service manifest.", color.HighlightCode(`"http.alias.hosted_zone"`)),
		fmt.Sprintf(" 3. Redeploying the service via %s.", color.HighlightCode("copilot svc deploy")),
	}
	return strings.Join(msgs, "\n")
}

type errEnvHasPublicServicesWithRedirect struct {
	services []string
}

func (e *errEnvHasPublicServicesWithRedirect) Error() string {
	return fmt.Sprintf("%v %s HTTP to HTTPS",
		len(e.services),
		english.PluralWord(len(e.services), "service redirects", "services redirect"),
	)
}

// RecommendActions returns recommended actions to be taken after the error.
// Implements main.actionRecommender interface.
func (e *errEnvHasPublicServicesWithRedirect) RecommendActions() string {
	n := len(e.services)
	quoted := make([]string, len(e.services))
	for i := range e.services {
		quoted[i] = strconv.Quote(e.services[i])
	}

	return fmt.Sprintf(`%s %s %s HTTP traffic to HTTPS.
%s
To fix this, set the following field in %s manifest:
%s
and run %s.`,
		english.PluralWord(n, "Service", "Services"),
		english.OxfordWordSeries(quoted, "and"),
		english.PluralWord(n, "redirects", "redirect"),
		color.Emphasize(english.PluralWord(n, "This service", "These services")+" will not be reachable through the CDN."),
		english.PluralWord(n, "its", "each"),
		color.HighlightCodeBlock("http:\n  redirect_to_https: false"),
		color.HighlightCode("copilot svc deploy"),
	)
}

func (e *errEnvHasPublicServicesWithRedirect) warning() string {
	return fmt.Sprintf(`%s
If you'd like to use %s without a CDN, ensure %s A record is pointed to the ALB.`,
		e.RecommendActions(),
		english.PluralWord(len(e.services), "this service", "these services"),
		english.PluralWord(len(e.services), "its", "each service's"),
	)
}
