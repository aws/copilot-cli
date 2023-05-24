// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package staticsite_test

import (
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

var _ = Describe("Static Site", func() {
	Context("when creating a new app", Ordered, func() {
		var appInitErr error

		BeforeAll(func() {
			_, appInitErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
				Domain:  domainName,
			})
		})

		It("app init succeeds", func() {
			Expect(appInitErr).NotTo(HaveOccurred())
		})

		It("app init creates a copilot directory", func() {
			Expect("./copilot").Should(BeADirectory())
		})

		It("app ls includes new application", func() {
			Eventually(cli.AppList, "30s", "5s").Should(ContainSubstring(appName))
		})

		It("app show includes domain name", func() {
			appShowOutput, err := cli.AppShow(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(appShowOutput.Name).To(Equal(appName))
			Expect(appShowOutput.URI).To(Equal(domainName))
		})
	})

	Context("when adding new environment", Ordered, func() {
		var err error
		BeforeAll(func() {
			_, err = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: "default",
			})
		})
		It("env init should succeed", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environments", Ordered, func() {
		var envDeployErr error
		BeforeAll(func() {
			_, envDeployErr = cli.EnvDeploy(&client.EnvDeployRequest{
				AppName: appName,
				Name:    "test",
			})
		})
		It("env deploy should succeed", func() {
			Expect(envDeployErr).NotTo(HaveOccurred())
		})
	})

	Context("when initializing Static Site", Ordered, func() {
		var svcInitErr error
		BeforeAll(func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:    "static",
				SvcType: "Static Site",
			})
		})
		It("svc init should succeed", func() {
			Expect(svcInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying a Static Site", func() {
		It("deployment should succeed", func() {
			_, err := cli.SvcDeploy(&client.SvcDeployInput{
				Name:    "static",
				EnvName: "test",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("svc show should contain the expected domain and the request should succeed", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				Name:    "static",
				AppName: appName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))
			route := svc.Routes[0]
			wantedURLs := map[string]string{
				"test": fmt.Sprintf("https://%s.%s", appName, domainName),
			}
			// Validate route has the expected HTTPS endpoint.
			Expect(route.URL).To(ContainSubstring(wantedURLs[route.Environment]))

			// Make sure the service response is OK.
			var resp *http.Response
			var fetchErr error
			Eventually(func() (int, error) {
				resp, fetchErr = http.Get(route.URL)
				return resp.StatusCode, fetchErr
			}, "60s", "1s").Should(Equal(200))
			// HTTP should work.
			Eventually(func() (int, error) {
				resp, fetchErr = http.Get(strings.Replace(route.URL, "https", "http", 1))
				return resp.StatusCode, fetchErr
			}, "60s", "1s").Should(Equal(200))
			// Make sure we route to index.html for sub-path.
			Eventually(func() (int, error) {
				resp, fetchErr = http.Get(fmt.Sprintf("%s/static", route.URL))
				return resp.StatusCode, fetchErr
			}, "60s", "1s").Should(Equal(200))
		})
	})
})
