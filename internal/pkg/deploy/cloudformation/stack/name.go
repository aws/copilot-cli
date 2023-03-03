// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strings"
)

const (
	// taskStackPrefix is used elsewhere to list CF stacks
	taskStackPrefix = "task-"

	// After v1.16, pipeline stack names are namespaced with a prefix of "pipeline-${appName}-".
	fmtPipelineNamespaced = "pipeline-%s-%s"
)

// TaskStackName holds the name of a Copilot one-off task stack.
type TaskStackName string

// TaskName returns the name of the task family, generated from the stack name
func (t TaskStackName) TaskName() string {
	return strings.SplitN(string(t), "-", 2)[1]
}

// NameForWorkload returns the stack name for a workload.
func NameForWorkload(app, env, name string) string {
	// stack name limit constrained by CFN https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cfn-using-console-create-stack-parameters.html
	const maxLen = 128
	stackName := fmt.Sprintf("%s-%s-%s", app, env, name)

	if len(stackName) > maxLen {
		return stackName[:maxLen]
	}
	return stackName
}

// NameForEnv returns the stack name for an environment.
func NameForEnv(app, env string) string {
	return fmt.Sprintf("%s-%s", app, env)
}

// NameForTask returns the stack name for a task.
func NameForTask(task string) TaskStackName {
	return TaskStackName(taskStackPrefix + task)
}

// NameForAppStack returns the stack name for an app.
func NameForAppStack(app string) string {
	return fmt.Sprintf("%s-infrastructure-roles", app)
}

// NameForAppStackSet returns the stackset name for an app.
func NameForAppStackSet(app string) string {
	return fmt.Sprintf("%s-infrastructure", app)
}

// NameForPipeline returns the stack name for a pipeline, depending on whether it has been deployed using the legacy scheme.
// Note that it doesn't cut name to length of 128 like service stack name. It expects CloudFormation to error out
// when the name is to long.
func NameForPipeline(app string, pipeline string, isLegacy bool) string {
	if isLegacy {
		return pipeline
	}
	return fmt.Sprintf(fmtPipelineNamespaced, app, pipeline)
}
