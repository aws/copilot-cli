// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package multi_svc_app_test

import (
       "errors"
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

       appName = fmt.Sprintf("regression-multisvcapp-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
       _, err := toCLI.Run("app", "delete", "--yes")
       Expect(err).NotTo(HaveOccurred())

       // Best-effort to swap back the files.
       for _, svcName := range []string{"front-end", "www", "back-end"} {
              if _, err := os.Stat(fmt.Sprintf("%s/swap/main.tmp", svcName)); errors.Is(err, os.ErrNotExist) {
                     // It's likely that a swap did not happen in the first place. No need to swap back.
                     continue
              }
              _ = os.Rename(fmt.Sprintf("%s/main.go", svcName), fmt.Sprintf("%s/swap/main.go", svcName))
              _ = os.Rename(fmt.Sprintf("%s/swap/main.tmp", svcName), fmt.Sprintf("%s/main.go", svcName))
       }
       if _, err := os.Stat("query/swap/entrypoint.tmp"); errors.Is(err, os.ErrNotExist) {
              return
       }
       _ = os.Rename("query/entrypoint.sh", "query/swap/entrypoint.sh")
       _ = os.Rename("query/swap/entrypoint.tmp", "query/entrypoint.sh")
})
