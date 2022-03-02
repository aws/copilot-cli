// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package apprunner_test

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

const feSvcName = "front-end"
const beSvcName = "back-end"
const envName = "test"

/**
The Init Suite runs through the copilot init workflow for a brand new
application. It creates a single environment, deploys a service to it, and then
tears it down.
*/
func TestInit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "App Runner Suite")
}

var _ = BeforeSuite(func() {
	var err error
	cli, err = client.NewCLI()
	Expect(err).NotTo(HaveOccurred())
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("e2e-apprunner-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete()
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
