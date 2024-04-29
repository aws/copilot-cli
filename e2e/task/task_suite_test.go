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
var appName, envName string
var tasks []client.TaskRunInput

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
})

var _ = AfterSuite(func() {
	for _, task := range tasks {
		_, err := cli.TaskDelete(&client.TaskDeleteInput{
			App:     task.AppName,
			Env:     task.Env,
			Name:    task.GroupName,
			Default: task.Default,
		})
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("delete task %s", task.GroupName))
	}
	_, err := cli.AppDelete()
	Expect(err).NotTo(HaveOccurred(), "delete Copilot application")
})
