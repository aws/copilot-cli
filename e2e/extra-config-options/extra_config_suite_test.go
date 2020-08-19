// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package extra_config_test

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
var svcName string

// The Extra Config suite runs creates a new application with overrides specified in the manifest
func TestExtraConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extra Config Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("e2e-extra-config-%d", time.Now().Unix())
	svcName = "docker-mft"
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete(map[string]string{"test": "default"})
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
