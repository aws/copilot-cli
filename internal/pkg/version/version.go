// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package version holds variables for generating version information
package version

const (
	// LegacyAppTemplateVersion is the version associated with the application template before we started versioning.
	LegacyAppTemplateVersion = "v0.0.0"
	// AliasLeastAppTemplateVersion is the least version number available for HTTPS alias.
	AliasLeastAppTemplateVersion = "v1.0.0"
	// StaticSiteMinAppTemplateVersion is the minimum app version required to deploy a static site.
	StaticSiteMinAppTemplateVersion = "v1.2.0"

	defaultTemplateVersion = "v1.29.0" // v1.29.0 is when we use Copilot version to version templates.
)

// Version is this binary's version. Set with linker flags when building Copilot.
var Version string

// LatestTemplateVersion is the latest version number available for Copilot templates.
func LatestTemplateVersion() string {
	if Version != "" {
		return Version
	}
	// Fallback to defaultTemplateVersion. Theoretically this only happens in tests.
	return defaultTemplateVersion
}
