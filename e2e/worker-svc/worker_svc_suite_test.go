// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package worker_svc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var aws *client.AWS
var (
	appName       string
	envName       string
	workerName    string
	publisherName string
)

/**
The worker svc suite runs through several tests focusing on creating worker services
and publishers in one app.
*/
func TestWorkerSvcApp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Svc Suite (one workspace)")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	aws = client.NewAWS()
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("e2e-workersvc-%d", time.Now().Unix())
	envName = "test"
	workerName = "worker"
	publisherName = "publisher"
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
