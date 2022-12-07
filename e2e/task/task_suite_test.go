// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var aws *client.AWS
var appName, envName, groupName, taskStackName, repoName string

/**
The task suite runs through several tests focusing on running one-off tasks with different configurations.
*/
func TestTask(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Task Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	aws = client.NewAWS()

	appName = fmt.Sprintf("e2e-task-%d", time.Now().Unix())
	envName = "test"
	groupName = fmt.Sprintf("e2e-task-%d", time.Now().Unix())
	// We name task stack in format of "task-${groupName}".
	// See https://github.com/aws/copilot-cli/blob/e9e3114561e740c367fb83b5e075750f232ad639/internal/pkg/deploy/cloudformation/stack/name.go#L26.
	taskStackName = fmt.Sprintf("task-%s", groupName)
	// We name ECR repo name in format of "copilot-${groupName}".
	// See https://github.com/aws/copilot-cli/blob/e9e3114561e740c367fb83b5e075750f232ad639/templates/task/cf.yml#L75.
	repoName = fmt.Sprintf("copilot-%s", groupName)
})

var _ = AfterSuite(func() {
	_, err := cli.TaskDelete(&client.TaskDeleteInput{
		App:     appName,
		Env:     envName,
		Name:    groupName,
		Default: false,
	})
	Expect(err).NotTo(HaveOccurred(), "delete task")
	_, err = cli.AppDelete()
	Expect(err).NotTo(HaveOccurred(), "delete Copilot application")
})
