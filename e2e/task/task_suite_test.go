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
var groupNames []string

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
	for _, groupName := range groupNames {
		_, err := cli.TaskDelete(&client.TaskDeleteInput{
			App:     appName,
			Env:     envName,
			Name:    groupName,
			Default: false,
		})
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("delete task %s", groupName))
	}
	_, err := cli.AppDelete()
	Expect(err).NotTo(HaveOccurred(), "delete Copilot application")
})
