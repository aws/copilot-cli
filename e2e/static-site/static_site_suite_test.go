// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package static_site_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var appName string

const domainName = "static-site.copilot-e2e-tests.ecs.aws.dev"

var timeNow = time.Now().Unix()

func TestStaticSite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Static Site Suite")
}

var _ = BeforeSuite(func() {
	copilotCLI, err := client.NewCLI()
	Expect(err).NotTo(HaveOccurred())
	cli = copilotCLI
	appName = fmt.Sprintf("t%d", timeNow)
	err = os.Setenv("DOMAINNAME", domainName)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {})
