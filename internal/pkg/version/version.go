// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package version holds variables for generating version information
package version

const (
	// LegacyAppTemplate is the version associated with the application template before we started versioning.
	LegacyAppTemplate = "v0.0.0"
	// AppTemplateMinAlias is the least version number available for HTTPS alias.
	AppTemplateMinAlias = "v1.0.0"
	// AppTemplateMinStaticSite is the minimum app version required to deploy a static site.
	AppTemplateMinStaticSite = "v1.2.0"
)

// Version is this binary's version. Set with linker flags when building Copilot.
var Version string

// LatestTemplateVersion is the latest version number available for Copilot templates.
func LatestTemplateVersion() string {
	return Version
}
