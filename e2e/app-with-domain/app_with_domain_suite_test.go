// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package app_with_domain_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	waitingInterval = 60 * time.Second
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
	appName = fmt.Sprintf("e2e-domain-%d", time.Now().Unix())
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

func isOperationInProgress(s string) bool {
	return strings.Contains(s, cloudformation.ErrCodeOperationInProgressException)
}

func isImagePushingInProgress(s string) bool {
	return strings.Contains(s, "denied: Your authorization token has expired. Reauthenticate and try again.")
}
