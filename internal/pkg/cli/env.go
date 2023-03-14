// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

func validateMinEnvVersion(ws wsEnvironmentsLister, checker versionCompatibilityChecker, app, env, minWantedVersion, friendlyFeatureName string) error {
	version, err := checker.Version()
	if err != nil {
		return fmt.Errorf("retrieve version of environment stack %q in application %q: %v", env, app, err)
	}
	if semver.Compare(version, minWantedVersion) < 0 {
		return &errFeatureIncompatibleWithEnvironment{
			ws:             ws,
			missingFeature: friendlyFeatureName,
			envName:        env,
			curVersion:     version,
		}
	}
	return nil
}

// BuildEnvCmd is the top level command for environments.
func BuildEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "env",
		Short: `Commands for environments.
Environments are deployment stages shared between services.`,
		Long: `Commands for environments.
Environments are deployment stages shared between services.`,
	}

	cmd.AddCommand(buildEnvInitCmd())
	cmd.AddCommand(buildEnvListCmd())
	cmd.AddCommand(buildEnvShowCmd())
	cmd.AddCommand(buildEnvUpgradeCmd())
	cmd.AddCommand(buildEnvPkgCmd())
	cmd.AddCommand(buildEnvOverrideCmd())
	cmd.AddCommand(buildEnvDeployCmd())
	cmd.AddCommand(buildEnvDeleteCmd())
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}
