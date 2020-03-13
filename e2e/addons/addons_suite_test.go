// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var projectName string
var appName string

// The Addons suite runs creates a new application with additional resources.
func TestAddons(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Addons Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	projectName = fmt.Sprintf("e2e-addons-%d", time.Now().Unix())
	appName = "hello"
})

var _ = AfterSuite(func() {
	_, err := cli.ProjectDelete(map[string]string{"test": "default"})
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
