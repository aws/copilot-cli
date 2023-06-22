// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/version"
)

type errCannotDowngradeAppVersion struct {
	appName    string
	appVersion string
}

func (e *errCannotDowngradeAppVersion) init() *errCannotDowngradeVersion {
	return &errCannotDowngradeVersion{
		componentName:         e.appName,
		componentType:         "application",
		currentVersion:        e.appVersion,
		latestTemplateVersion: version.LatestTemplateVersion(),
	}
}

func (e *errCannotDowngradeAppVersion) Error() string {
	return e.init().Error()
}

func (e *errCannotDowngradeAppVersion) RecommendActions() string {
	return e.init().RecommendActions()
}

type errCannotDowngradeVersion struct {
	componentName         string
	componentType         string
	currentVersion        string
	latestTemplateVersion string
}

func (e *errCannotDowngradeVersion) Error() string {
	return fmt.Sprintf("cannot downgrade %s %q (currently in version %s) to version %s", e.componentType, e.componentName, e.currentVersion, e.latestTemplateVersion)
}

func (e *errCannotDowngradeVersion) RecommendActions() string {
	return fmt.Sprintf(`It looks like you are trying to use an earlier version of Copilot to downgrade %s lastly updated by a newer version of Copilot.
- We recommend upgrade your local Copilot CLI version and run this command again.
- Alternatively, you can run with %s to override. However, this can cause unsuccessful deployment. Please use with caution!`,
		color.HighlightCode(fmt.Sprintf("%s %s", e.componentType, e.componentName)), color.HighlightCode(fmt.Sprintf("--%s", allowDowngradeFlag)))
}
