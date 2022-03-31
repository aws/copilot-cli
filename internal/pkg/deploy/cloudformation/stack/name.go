// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strings"
)

// taskStackPrefix is used elsewhere to list CF stacks
const taskStackPrefix = "task-"

// After v1.16, pipeline names are namespaced with a prefix of "pipeline-${appName}-".
const fmtPipelineNamespaced = "pipeline-%s-%s"

// TaskStackName holds the name of a Copilot one-off task stack.
type TaskStackName string

// TaskName returns the name of the task family, generated from the stack name
func (t TaskStackName) TaskName() string {
	return strings.SplitN(string(t), "-", 2)[1]
}

// NameForService returns the stack name for a service.
func NameForService(app, env, svc string) string {
	// stack name limit constrained by CFN https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cfn-using-console-create-stack-parameters.html
	const maxLen = 128
	stackName := fmt.Sprintf("%s-%s-%s", app, env, svc)

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

// NameForNamespacedPipeline returns the stack name for a pipeline with namespaced naming scheme.
func NameForNamespacedPipeline(app string, pipeline string) string {
	return fmt.Sprintf(fmtPipelineNamespaced, app, pipeline)
}
