// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var (
	appName string
	svcName string

	rdsStorageName string
	rdsStorageType string
	rdsEngine      string
	rdsInitialDB   string

	s3StorageName string
	s3StorageType string
)

// The Addons suite runs creates a new application with additional resources.
func TestAddons(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Addons Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())

	appName = fmt.Sprintf("e2e-addons-%d", time.Now().Unix())
	svcName = "hello"

	rdsStorageName = "mycluster"
	rdsStorageType = "Aurora"
	rdsEngine = "PostgreSQL"
	rdsInitialDB = "initdb"

	s3StorageName = "mybucket"
	s3StorageType = "S3"
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete()
	_ = client.NewAWS().DeleteAllDBClusterSnapshots()
	Expect(err).NotTo(HaveOccurred())
})
