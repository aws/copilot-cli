// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package sidecars_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/e2e/internal/client"
	"github.com/aws/amazon-ecs-cli-v2/e2e/internal/command"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var appName string
var svcName string
var sidecarImageURI string
var sidecarRepoName string

// The Sidecars suite runs creates a new service with sidecar containers.
func TestSidecars(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sidecars Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("e2e-sidecars-%d", time.Now().Unix())
	svcName = "hello"
	sidecarRepoName = fmt.Sprintf("e2e-sidecars-nginx-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete(map[string]string{"test": "default"})
	Expect(err).NotTo(HaveOccurred())
	err = command.Run("aws", []string{"ecr", "delete-repository", "--repository-name", sidecarRepoName, "--force"})
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
