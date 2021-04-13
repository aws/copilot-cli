// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var (
	appName string
	svcName string

	storageName  string
	storageType  string
	rdsEngine    string
	rdsInitialDB string
)

// The Addons suite runs creates a new application with additional resources.
func TestAddons(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Addons RDS Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())

	appName = fmt.Sprintf("e2e-addons-%d", time.Now().Unix())
	svcName = "hello"

	storageName = "mystorage"
	storageType = "Aurora"
	rdsEngine = "PostgreSQL"
	rdsInitialDB = "initdb"
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
