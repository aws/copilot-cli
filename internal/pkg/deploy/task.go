// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines service deployment resources.
package deploy

import (
	"fmt"
	"strings"
)

// FmtTaskECRRepoName is the pattern used to generate the ECR repository's name
const FmtTaskECRRepoName = "copilot-%s"

// CreateTaskResourcesInput holds the fields required to create a task stack.
type CreateTaskResourcesInput struct {
	Name   string
	CPU    int
	Memory int

	Image                 string
	PermissionsBoundary   string
	TaskRole              string
	ExecutionRole         string
	Command               []string
	EntryPoint            []string
	EnvVars               map[string]string
	EnvFileARN            string
	SSMParamSecrets       map[string]string
	SecretsManagerSecrets map[string]string

	OS   string
	Arch string

	App string
	Env string

	AdditionalTags map[string]string
}

// TaskStackInfo contains essential information about a Copilot task stack
type TaskStackInfo struct {
	StackName string
	App       string
	Env       string

	RoleARN string

	BucketName string
}

// TaskName returns the name of the one-off task. This is the same as the value of the
// copilot-task tag. For example, a stack called "task-db-migrate" will have the TaskName "db-migrate"
func (t TaskStackInfo) TaskName() string {
	return strings.SplitN(t.StackName, "-", 2)[1]
}

// ECRRepoName returns the name of the ECR repo for the one-off task.
func (t TaskStackInfo) ECRRepoName() string {
	return fmt.Sprintf(FmtTaskECRRepoName, t.TaskName())
}
