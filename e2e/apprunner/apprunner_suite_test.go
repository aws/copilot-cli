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

/*
*
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
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("e2e-apprunner-%d", time.Now().Unix())
	err = command.Run("aws", []string{"ssm", "put-parameter", "--name", ssmName, "--overwrite", "--value", "abcd1234", "--type", "String", "--tags", "[{\"Key\":\"copilot-application\",\"Value\":\"" + appName + "\"},{\"Key\":\"copilot-environment\", \"Value\":\"" + envName + "\"}]"})
	Expect(err).NotTo(HaveOccurred())
	err = command.Run("aws", []string{"secretsmanager", "create-secret", "--name", secretName, "--force-overwrite-replica-secret", "--secret-string", "\"{\"user\":\"diegor\",\"password\":\"EXAMPLE-PASSWORD\"}\"", "--tags", "[{\"Key\":\"copilot-application\",\"Value\":\"" + appName + "\"},{\"Key\":\"copilot-environment\", \"Value\":\"" + envName + "\"}]"})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete()
	Expect(err).NotTo(HaveOccurred())
	err = command.Run("aws", []string{"ssm", "delete-parameter", "--name", "e2e-apprunner-ssm-param"})
	Expect(err).NotTo(HaveOccurred())
	err = command.Run("aws", []string{"secretsmanager", "delete-secret", "--secret-id", "e2e-apprunner-MyTestSecret", "--force-delete-without-recovery"})
	Expect(err).NotTo(HaveOccurred())
	_ = client.NewAWS().DeleteAllDBClusterSnapshots()
})
