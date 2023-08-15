// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package static_site_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

func testResponse(url string) {
	// Make sure the service response is OK.
	var resp *http.Response
	var fetchErr error
	resp, fetchErr = http.Get(url)
	Expect(fetchErr).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(200))
	bodyBytes, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	Expect(string(bodyBytes)).To(Equal("hello"))

	// HTTP should work.
	resp, fetchErr = http.Get(strings.Replace(url, "https", "http", 1))
	Expect(fetchErr).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(200))

	// Make sure we route to index.html for sub-path.
	resp, fetchErr = http.Get(fmt.Sprintf("%s/static", url))
	Expect(fetchErr).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(200))
	bodyBytes, err = io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	Expect(string(bodyBytes)).To(Equal("bye"))
}
