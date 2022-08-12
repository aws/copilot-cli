// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package import_certs

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
var testEnvironmentProfile string

func TestAppWithDomain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Import Certificates Suite")
}

var _ = BeforeSuite(func() {
	testEnvironmentProfile = "e2etestenv"
	copilotCLI, err := client.NewCLI()
	cli = copilotCLI
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("e2e-import-certs-%d", time.Now().Unix())
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
