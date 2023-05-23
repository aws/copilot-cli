// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package apprunner_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	"github.com/aws/copilot-cli/e2e/internal/command"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var appName string

var ssmName string
var secretName string

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

	appName = fmt.Sprintf("e2e-apprunner-%d", time.Now().Unix())
	ssmName = fmt.Sprintf("%s-%s", appName, "ssm")
	err = os.Setenv("SSM_NAME", ssmName)
	Expect(err).NotTo(HaveOccurred())
	secretName = fmt.Sprintf("%s-%s", appName, "secretsmanager")
	err = os.Setenv("SECRETS_MANAGER_NAME", secretName)
	Expect(err).NotTo(HaveOccurred())

	err = command.Run("aws", []string{"ssm", "put-parameter", "--name", ssmName, "--overwrite", "--value", "abcd1234", "--type", "String"})
	Expect(err).NotTo(HaveOccurred())
	err = command.Run("aws", []string{"ssm", "add-tags-to-resource", "--resource-type", "Parameter", "--resource-id", ssmName, "--tags", "[{\"Key\":\"copilot-application\",\"Value\":\"" + appName + "\"},{\"Key\":\"copilot-environment\", \"Value\":\"" + envName + "\"}]"})
	Expect(err).NotTo(HaveOccurred())
	_ = command.Run("aws", []string{"secretsmanager", "create-secret", "--name", secretName, "--secret-string", "\"{\"user\":\"diegor\",\"password\":\"EXAMPLE-PASSWORD\"}\"", "--tags", "[{\"Key\":\"copilot-application\",\"Value\":\"" + appName + "\"},{\"Key\":\"copilot-environment\", \"Value\":\"" + envName + "\"}]"})
})

var _ = AfterSuite(func() {
	_, appDeleteErr := cli.AppDelete()
	ssmDeleteErr := command.Run("aws", []string{"ssm", "delete-parameter", "--name", ssmName})
	secretsDeleteErr := command.Run("aws", []string{"secretsmanager", "delete-secret", "--secret-id", secretName, "--force-delete-without-recovery"})
	_ = client.NewAWS().DeleteAllDBClusterSnapshots()
	Expect(appDeleteErr).NotTo(HaveOccurred())
	Expect(ssmDeleteErr).NotTo(HaveOccurred())
	Expect(secretsDeleteErr).NotTo(HaveOccurred())
})
