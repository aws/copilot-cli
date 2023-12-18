// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/dustin/go-humanize/english"
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

type errNoInfrastructureChanges struct {
	parentErr error
}

func (e *errNoInfrastructureChanges) Error() string {
	return e.parentErr.Error()
}

func (e *errNoInfrastructureChanges) ExitCode() int {
	return 0
}

type errBucketEmptyingFailed struct {
	failedBuckets []string
	bucketErrors  []error
}

func (e *errBucketEmptyingFailed) Error() string {
	return fmt.Sprintf("emptying %v %v failed: %v", english.PluralWord(len(e.failedBuckets), "bucket", "buckets"),
		english.WordSeries(e.failedBuckets, "and"), errors.Join(e.bucketErrors...))
}

func (e *errBucketEmptyingFailed) RecommendActions() string {
	return fmt.Sprintf(`Copilot failed to empty and delete %v managed by your environment. The %v now a dangling resource.
- We recommend logging into the S3 console and manually deleting the affected %v.`,
		english.PluralWord(len(e.failedBuckets), "an S3 bucket", "S3 buckets"), english.PluralWord(len(e.failedBuckets), "bucket is", "buckets are"), english.PluralWord(len(e.failedBuckets), "bucket", "buckets"))
}

type errPipelineDependsOnEnv struct {
	pipeline string
	env      string
}

func (e *errPipelineDependsOnEnv) Error() string {
	return fmt.Sprintf("environment %q cannot be deleted because pipeline %q depends on it", e.env, e.pipeline)
}

func (e *errPipelineDependsOnEnv) RecommendActions() string {
	return fmt.Sprintf(`You can update the manifest of the pipeline %q to remove its dependency on environment %q,
or run %s to delete the pipeline before running %s to delete the environment`,
		e.pipeline, e.env, color.HighlightCode(fmt.Sprintf("copilot pipeline delete -n %s", e.pipeline)), color.HighlightCode(fmt.Sprintf("copilot env delete -n %s", e.env)))
}

type errTaskRoleRetrievalFailed struct {
	chainErrs []error
}

func (e *errTaskRoleRetrievalFailed) Error() string {
	return errors.Join(e.chainErrs...).Error()
}

func (e *errTaskRoleRetrievalFailed) RecommendActions() string {
	return fmt.Sprintf(`TaskRole retrieval failed. If your containers don't require the TaskRole for local testing, you can use %s to disable this feature.
If you require the TaskRole, you can manually add permissions for your account to assume TaskRole by adding the following YAML override to your service:
%s
For more information on YAML overrides see %s`,
		color.HighlightCode(`copilot run local --use-task-role=false`),
		color.HighlightCodeBlock(`- op: add
  path: /Resources/TaskRole/Properties/AssumeRolePolicyDocument/Statement/-
  value:
    Effect: Allow
    Principal:
      AWS: "arn:aws:iam::[app-account-ID]:root"
    Action: 'sts:AssumeRole'`),
		color.Emphasize("https://aws.github.io/copilot-cli/docs/developing/overrides/yamlpatch/"))
}
