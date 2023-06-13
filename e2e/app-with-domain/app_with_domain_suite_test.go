// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package app_with_domain_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	waitingInterval = 60 * time.Second
	domainName      = "app-with-domain.copilot-e2e-tests.ecs.aws.dev"
)

var cli *client.CLI
var appName string

func TestAppWithDomain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Custom Domain Suite")
}

var _ = BeforeSuite(func() {
	copilotCLI, err := client.NewCLI()
	cli = copilotCLI
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("DOMAINNAME", domainName)
	Expect(err).NotTo(HaveOccurred())
	appName = fmt.Sprintf("t%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete()
	Expect(err).NotTo(HaveOccurred())
})

// isStackSetOperationInProgress returns if the current stack set is in operation.
func isStackSetOperationInProgress(s string) bool {
	return strings.Contains(s, cloudformation.ErrCodeOperationInProgressException)
}

// isImagePushingToECRInProgress returns if we are pushing images to ECR. Pushing images concurrently would fail because
// of credential verification issue.
func isImagePushingToECRInProgress(s string) bool {
	return strings.Contains(s, "denied: Your authorization token has expired. Reauthenticate and try again.") ||
		strings.Contains(s, "no basic auth credentials")
}
