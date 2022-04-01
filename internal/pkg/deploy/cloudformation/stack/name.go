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

	maxStackNameLength      = 128
	minChoppedAppNameLength = 7
)

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

// NameForPipeline returns the stack name for a pipeline, depending on whether it has been deployed using the legacy scheme.
// It keeps the stack name under 128 by first attempting to chop the app name to length of 7, and then chop pipeline name
// so that the total length is 128.
func NameForPipeline(app string, pipeline string, isLegacy bool) string {
	if isLegacy {
		return pipeline
	}
	raw := fmt.Sprintf(fmtPipelineNamespaced, app, pipeline)
	if len(raw) <= maxStackNameLength {
		return raw
	}
	lenToChop := len(raw) - maxStackNameLength
	choppedApp := cutNFromHead(app, len(app)-7)

	lenToChop = lenToChop - (len(app) - len(choppedApp))
	choppedPipeline := smartCutNFromString(pipeline, lenToChop)
	return fmt.Sprintf(fmtPipelineNamespaced, choppedApp, choppedPipeline)
}

func cutNFromHead(s string, n int) string {
	if n <= 0 {
		return s
	}
	if len(s) <= n {
		return ""
	}
	return s[n:]
}

func smartCutNFromString(s string, n int) string {
	if n <= 0 {
		return s
	}
	if len(s) <= n {
		return ""
	}

	// If we need to cut more than 1/3, we just cut from head.
	if n > len(s)/3 {
		return cutNFromHead(s, n)
	}

	head := len(s) / 3
	if tail := len(s) - (head + n); tail < 7 {
		return cutNFromHead(s, n) // If too little consecutive letters are preserved at the end, we just cut from head.
	}
	chopped := s[:head] + s[head+n:]
	return chopped
}
