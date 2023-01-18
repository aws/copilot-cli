// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package multi_svc_app_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/copilot-cli/regression/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	toCLI   *client.CLI
	fromCLI *client.CLI

	appName string
)

// The Addons suite runs creates a new application with additional resources.
func TestMultiSvcApp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Regression For Multi Svc App Suite")
}

var _ = BeforeSuite(func() {
	var err error

	toCLI, err = client.NewCLI(os.Getenv("REGRESSION_TEST_TO_PATH"))
	Expect(err).NotTo(HaveOccurred())

	fromCLI, err = client.NewCLI(os.Getenv("REGRESSION_TEST_FROM_PATH"))
	Expect(err).NotTo(HaveOccurred())

	appName = fmt.Sprintf("regression-multisvcapp-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, err := toCLI.Run("app", "delete", "--yes")
	Expect(err).NotTo(HaveOccurred())
})
