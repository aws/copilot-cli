// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package sidecars_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	"github.com/aws/copilot-cli/e2e/internal/command"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var aws *client.AWS
var docker *client.Docker
var appName string
var envName string
var svcName string
var mainRepoName string

// The Sidecars suite runs creates a new service with sidecar containers.
func TestSidecars(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sidecars Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	aws = client.NewAWS()
	docker = client.NewDocker()
	appName = fmt.Sprintf("e2e-sidecars-%d", time.Now().Unix())
	envName = "test"
	svcName = "hello"
	mainRepoName = fmt.Sprintf("e2e-sidecars-main-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, appDeleteErr := cli.AppDelete()
	repoDeleteErr := command.Run("aws", []string{"ecr", "delete-repository", "--repository-name", mainRepoName, "--force"})
	Expect(appDeleteErr).NotTo(HaveOccurred())
	Expect(repoDeleteErr).NotTo(HaveOccurred())
})

// exponentialBackoffWithJitter backoff exponentially with jitter based on 200ms base
// component of backoff fixed to ensure minimum total wait time on
// slow targets.
func exponentialBackoffWithJitter(attempt int) {
	base := int(math.Pow(2, float64(attempt)))
	time.Sleep(time.Duration((rand.Intn(50)*base + base*150) * int(time.Millisecond)))
}
