// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package app_with_domain_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

var _ = Describe("App With Domain", func() {
	const domainName = "copilot-e2e-tests.ecs.aws.dev"
	const svcName = "hello"

	Context("when creating a new app", func() {
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

	Context("when creating a new environment", func() {
		var envInitErr error

		It("env init should succeed for creating the test environment", func() {
			_, envInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: "default",
				Prod:    false,
			})
			Expect(envInitErr).NotTo(HaveOccurred())
		})

		It("env init should succeed for creating the prod environment", func() {
			_, envInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "prod",
				Profile: "default",
				Prod:    false,
			})
			Expect(envInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when initializing a Load Balanced Web Service", func() {
		var svcInitErr error

		BeforeAll(func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       svcName,
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./hello/Dockerfile",
				SvcPort:    "80",
			})
		})

		It("svc init should succeed", func() {
			Expect(svcInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying a Load Balanced Web Service", func() {
		var deployErr error

		It("svc deploy to test should succeed", func() {
			_, deployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     svcName,
				EnvName:  "test",
				ImageTag: "hello",
			})
			Expect(deployErr).NotTo(HaveOccurred())
		})

		It("svc deploy to prod should succeed", func() {
			_, deployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     svcName,
				EnvName:  "prod",
				ImageTag: "hello",
			})
			Expect(deployErr).NotTo(HaveOccurred())
		})

		It("svc show should contain the expected domain and the request should succeed", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				Name:    svcName,
				AppName: appName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(2))

			wantedURLs := map[string]string{
				"test": "https://test.copilot-e2e-tests.ecs.aws.dev",
				"prod": "https://prod.copilot-e2e-tests.ecs.aws.dev",
			}
			for _, route := range svc.Routes {
				// Validate route has the expected HTTPS endpoint.
				Expect(route.URL).To(Equal(wantedURLs[route.Environment]))

				// Make sure the response is OK.
				var resp *http.Response
				var fetchErr error
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(route.URL)
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
				// HTTP should route to HTTPS.
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(route.URL)
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
			}
		})
	})
})
