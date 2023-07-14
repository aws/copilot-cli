// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

type errCannotDowngradePipelineVersion struct {
	name            string
	version         string
	templateVersion string
}

func (e *errCannotDowngradePipelineVersion) init() *errCannotDowngradeVersion {
	return &errCannotDowngradeVersion{
		componentName: e.name,
		componentType: "pipeline",
		laterVersion:  e.version,
		thisVersion:   e.templateVersion,
	}
}

func (e *errCannotDowngradePipelineVersion) Error() string {
	return e.init().Error()
}

func (e *errCannotDowngradePipelineVersion) RecommendActions() string {
	return e.init().RecommendActions()
}

type errCannotDowngradeWkldVersion struct {
	name            string
	version         string
	templateVersion string
}

func (e *errCannotDowngradeWkldVersion) init() *errCannotDowngradeVersion {
	return &errCannotDowngradeVersion{
		componentName: e.name,
		componentType: "workload",
		laterVersion:  e.version,
		thisVersion:   e.templateVersion,
	}
}

func (e *errCannotDowngradeWkldVersion) Error() string {
	return e.init().Error()
}

func (e *errCannotDowngradeWkldVersion) RecommendActions() string {
	return e.init().RecommendActions()
}

type errCannotDowngradeEnvVersion struct {
	envName         string
	envVersion      string
	templateVersion string
}

func (e *errCannotDowngradeEnvVersion) init() *errCannotDowngradeVersion {
	return &errCannotDowngradeVersion{
		componentName: e.envName,
		componentType: "environment",
		laterVersion:  e.envVersion,
		thisVersion:   e.templateVersion,
	}
}

func (e *errCannotDowngradeEnvVersion) Error() string {
	return e.init().Error()
}

func (e *errCannotDowngradeEnvVersion) RecommendActions() string {
	return e.init().RecommendActions()
}

type errCannotDowngradeAppVersion struct {
	appName         string
	appVersion      string
	templateVersion string
}

func (e *errCannotDowngradeAppVersion) init() *errCannotDowngradeVersion {
	return &errCannotDowngradeVersion{
		componentName: e.appName,
		componentType: "application",
		laterVersion:  e.appVersion,
		thisVersion:   e.templateVersion,
	}
}

func (e *errCannotDowngradeAppVersion) Error() string {
	return e.init().Error()
}

func (e *errCannotDowngradeAppVersion) RecommendActions() string {
	return e.init().RecommendActions()
}

type errCannotDowngradeVersion struct {
	componentName string
	componentType string
	laterVersion  string
	thisVersion   string
}

func (e *errCannotDowngradeVersion) Error() string {
	return fmt.Sprintf("cannot downgrade %s %q (currently in version %s) to version %s", e.componentType, e.componentName, e.laterVersion, e.thisVersion)
}

func (e *errCannotDowngradeVersion) RecommendActions() string {
	return fmt.Sprintf(`It looks like you are trying to use an earlier version of Copilot to downgrade %s lastly updated by a newer version of Copilot.
- We recommend upgrade your local Copilot CLI version and run this command again.
- Alternatively, you can run with %s to override. However, this can cause unsuccessful deployment. Please use with caution!`,
		color.HighlightCode(fmt.Sprintf("%s %s", e.componentType, e.componentName)), color.HighlightCode(fmt.Sprintf("--%s", allowDowngradeFlag)))
}
