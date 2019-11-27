// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import "fmt"

// NameForApp returns the stack name for an application.
func NameForApp(project, env, app string) string {
	// stack name limit constrained by CFN https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cfn-using-console-create-stack-parameters.html
	const maxLen = 128
	stackName := fmt.Sprintf("%s-%s-%s", project, env, app)

	if len(stackName) > maxLen {
		return stackName[:maxLen]
	}
	return stackName
}

// NameForEnv returns the stack name for an environment.
func NameForEnv(project, env string) string {
	return fmt.Sprintf("%s-%s", project, env)
}
