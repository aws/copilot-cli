// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package multi_svc_app_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/copilot-cli/regression/client"
	. "github.com/onsi/ginkgo"
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

	appName = fmt.Sprintf("regression-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, err := toCLI.Run("app", "delete", "--yes")
	Expect(err).NotTo(HaveOccurred())

	for _, svcName := range []string{"front-end", "www", "back-end"} {
		err := os.Rename(fmt.Sprintf("%s/main.go", svcName), fmt.Sprintf("%s/swap/main.go", svcName))
		Expect(err).NotTo(HaveOccurred())
		err = os.Rename(fmt.Sprintf("%s/swap/main.tmp", svcName), fmt.Sprintf("%s/main.go", svcName))
		Expect(err).NotTo(HaveOccurred())
	}
	err = os.Rename("query/entrypoint.sh", "query/swap/entrypoint.sh")
	Expect(err).NotTo(HaveOccurred())
	err = os.Rename("query/swap/entrypoint.tmp", "query/entrypoint.sh")
	Expect(err).NotTo(HaveOccurred())
})
