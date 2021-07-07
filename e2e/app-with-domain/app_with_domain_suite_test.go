// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package app_with_domain_test

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
var prodEnvironmentProfile string

func TestAppWithDomain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Multiple Svc Suite (one workspace)")
}

var _ = BeforeSuite(func() {
	prodEnvironmentProfile = "e2eprodenv"
	copilotCLI, err := client.NewCLI()
	cli = copilotCLI
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("e2e-domain-%d", time.Now().Unix())
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
