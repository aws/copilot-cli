// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package apprunner_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	"github.com/aws/copilot-cli/e2e/internal/command"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var appName string

const ssmName = "e2e-apprunner-ssm-param-secret"
const secretName = "e2e-apprunner-secrets-manager-secret"
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
	appName = fmt.Sprintf("e2e-apprunner-%d", time.Now().Unix())
	_ = command.Run("aws", []string{"ssm", "put-parameter", "--name", ssmName, "--value", "abcd1234", "--type", "String", "--tags", "[{\"Key\":\"copilot-application\",\"Value\":\"" + appName + "\"},{\"Key\":\"copilot-environment\", \"Value\":\"" + envName + "\"}]"})
	_ = command.Run("aws", []string{"secretsmanager", "create-secret", "--name", secretName, "--secret-string", "\"{\"user\":\"diegor\",\"password\":\"EXAMPLE-PASSWORD\"}\"", "--tags", "[{\"Key\":\"copilot-application\",\"Value\":\"" + appName + "\"},{\"Key\":\"copilot-environment\", \"Value\":\"" + envName + "\"}]"})
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete()
	Expect(err).NotTo(HaveOccurred())
	_ = command.Run("aws", []string{"ssm", "delete-parameter", "--name", ssmName})
	_ = command.Run("aws", []string{"secretsmanager", "delete-secret", "--secret-id", secretName, "--force-delete-without-recovery"})
	_ = client.NewAWS().DeleteAllDBClusterSnapshots()
})
