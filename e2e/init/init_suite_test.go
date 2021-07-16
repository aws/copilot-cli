// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package init_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var appName string
var envName string

/**
The Init Suite runs through the copilot init workflow for a brand new
application. It creates a single environment, deploys a service to it, and then
tears it down.
*/
func TestInit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Init Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("e2e-init-%d", time.Now().Unix())
	envName = "test"
})

var _ = AfterSuite(func() {
	_, err := cli.SvcDelete("front-end")
	Expect(err).NotTo(HaveOccurred())

	_, err = cli.JobDelete("mailer")
	Expect(err).NotTo(HaveOccurred())

	_, err = cli.EnvDelete("test")
	Expect(err).NotTo(HaveOccurred())

	_, err = cli.AppDelete()
	Expect(err).NotTo(HaveOccurred())
})

func BeforeAll(fn func()) {
	first := true
	BeforeEach(func() {
		if first {
			fn()
			first = false
		}
	})
}
