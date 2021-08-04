// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package app_with_domain_test

import (
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

var _ = Describe("App With Domain", func() {
	const domainName = "copilot-e2e-tests.ecs.aws.dev"

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

	Context("when creating new environments", func() {
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
				Profile: prodEnvironmentProfile,
				Prod:    false,
			})
			Expect(envInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when initializing Load Balanced Web Services", func() {
		var svcInitErr error

		It("svc init should succeed for creating the hello service", func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "hello",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./src/Dockerfile",
				SvcPort:    "80",
			})
			Expect(svcInitErr).NotTo(HaveOccurred())
		})

		It("svc init should succeed for creating the frontend service", func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "frontend",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./src/Dockerfile",
				SvcPort:    "80",
			})
			Expect(svcInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying a Load Balanced Web Service", func() {
		var deployErr error

		It("deploy hello to test should succeed", func() {
			_, deployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     "hello",
				EnvName:  "test",
				ImageTag: "hello",
			})
			Expect(deployErr).NotTo(HaveOccurred())
		})

		It("deploy frontend to test should succeed", func() {
			_, deployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     "frontend",
				EnvName:  "test",
				ImageTag: "frontend",
			})
			Expect(deployErr).NotTo(HaveOccurred())
		})

		It("deploy hello to prod should succeed", func() {
			_, deployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     "hello",
				EnvName:  "prod",
				ImageTag: "hello",
			})
			Expect(deployErr).NotTo(HaveOccurred())
		})

		It("svc show should contain the expected domain and the request should succeed", func() {
			// Check hello service
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				Name:    "hello",
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
					resp, fetchErr = http.Get(strings.Replace(route.URL, "https", "http", 1))
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
			}

			// Check frontend service.
			svc, err = cli.SvcShow(&client.SvcShowRequest{
				Name:    "frontend",
				AppName: appName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))
			wantedURLs = map[string]string{
				"test": "https://frontend.copilot-e2e-tests.ecs.aws.dev or https://copilot-e2e-tests.ecs.aws.dev",
			}
			// Validate route has the expected HTTPS endpoint.
			route := svc.Routes[0]
			Expect(route.URL).To(Equal(wantedURLs[route.Environment]))

			// Make sure the response is OK.
			var resp *http.Response
			var fetchErr error
			urls := strings.Split(route.URL, " or ")
			for _, url := range urls {
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(url)
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
				// HTTP should route to HTTPS.
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(strings.Replace(url, "https", "http", 1))
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
			}
		})
	})
})
