// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package app_with_domain_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

var _ = Describe("App With Domain", func() {
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

	Context("when adding new environments", func() {
		fatalErrors := make(chan error)
		wgDone := make(chan bool)
		It("env init should succeed for adding test and prod environments", func() {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				for {
					content, err := cli.EnvInit(&client.EnvInitRequest{
						AppName: appName,
						EnvName: "test",
						Profile: "test",
					})
					if err == nil {
						break
					}
					if !isStackSetOperationInProgress(content) {
						fatalErrors <- err
					}
					time.Sleep(waitingInterval)
				}
			}()
			go func() {
				defer wg.Done()
				for {
					content, err := cli.EnvInit(&client.EnvInitRequest{
						AppName: appName,
						EnvName: "prod",
						Profile: "prod",
					})
					if err == nil {
						break
					}
					if !isStackSetOperationInProgress(content) {
						fatalErrors <- err
					}
					time.Sleep(waitingInterval)
				}
			}()
			go func() {
				wg.Wait()
				close(wgDone)
				close(fatalErrors)
			}()

			select {
			case <-wgDone:
			case err := <-fatalErrors:
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Context("when deploying the environments", func() {
		fatalErrors := make(chan error)
		wgDone := make(chan bool)
		It("env deploy should succeed for deploying test and prod environments", func() {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, err := cli.EnvDeploy(&client.EnvDeployRequest{
					AppName: appName,
					Name:    "test",
				})
				if err != nil {
					fatalErrors <- err
				}
			}()
			go func() {
				defer wg.Done()
				_, err := cli.EnvDeploy(&client.EnvDeployRequest{
					AppName: appName,
					Name:    "prod",
				})
				if err != nil {
					fatalErrors <- err
				}
			}()
			go func() {
				wg.Wait()
				close(wgDone)
				close(fatalErrors)
			}()

			select {
			case <-wgDone:
			case err := <-fatalErrors:
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Context("when initializing Load Balanced Web Services", func() {
		var svcInitErr error

		It("svc init should succeed for creating the hello service", func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "hello",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./src/Dockerfile",
				SvcPort:    "3000",
			})
			Expect(svcInitErr).NotTo(HaveOccurred())
		})

		It("svc init should succeed for creating the frontend service", func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "frontend",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./src/Dockerfile",
				SvcPort:    "3000",
			})
			Expect(svcInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying a Load Balanced Web Service", func() {
		It("deployments should succeed", func() {
			fatalErrors := make(chan error)
			wgDone := make(chan bool)
			var wg sync.WaitGroup
			wg.Add(3)
			// deploy hello to test.
			go func() {
				defer wg.Done()
				for {
					content, err := cli.SvcDeploy(&client.SvcDeployInput{
						Name:     "hello",
						EnvName:  "test",
						ImageTag: "hello",
					})
					if err == nil {
						break
					}
					if !isStackSetOperationInProgress(content) && !isImagePushingToECRInProgress(content) {
						fatalErrors <- err
					}
					time.Sleep(waitingInterval)
				}
			}()
			// deploy frontend to test.
			go func() {
				defer wg.Done()
				for {
					content, err := cli.SvcDeploy(&client.SvcDeployInput{
						Name:     "frontend",
						EnvName:  "test",
						ImageTag: "frontend",
					})
					if err == nil {
						break
					}
					if !isStackSetOperationInProgress(content) && !isImagePushingToECRInProgress(content) {
						fatalErrors <- err
					}
					time.Sleep(waitingInterval)
				}
			}()
			// deploy hello to prod.
			go func() {
				defer wg.Done()
				for {
					content, err := cli.SvcDeploy(&client.SvcDeployInput{
						Name:     "hello",
						EnvName:  "prod",
						ImageTag: "hello",
					})
					if err == nil {
						break
					}
					if !isStackSetOperationInProgress(content) && !isImagePushingToECRInProgress(content) {
						fatalErrors <- err
					}
					time.Sleep(waitingInterval)
				}
			}()
			go func() {
				wg.Wait()
				close(wgDone)
				close(fatalErrors)
			}()

			select {
			case <-wgDone:
			case err := <-fatalErrors:
				Expect(err).NotTo(HaveOccurred())
			}
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
				"test": fmt.Sprintf("https://test.%s", domainName),
				"prod": fmt.Sprintf("https://prod.%s", domainName),
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
				"test": fmt.Sprintf("https://%s or https://frontend.%s", domainName, domainName),
			}
			// Validate route has the expected HTTPS endpoint.
			route := svc.Routes[0]
			Expect(route.URL).To(Equal(wantedURLs[route.Environment]))

			// Make sure the response is OK.
			var resp *http.Response
			var fetchErr error
			urls := strings.Split(route.URL, " or ")
			expectedRootResponseBody := "This is Copilot express app"
			expectedAdminResponseBody := "This is Copilot express app for admin"
			for _, url := range urls {
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(url)
					if fetchErr != nil {
						return 0, fetchErr
					}
					defer resp.Body.Close()
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return 0, err
					}
					if string(body) != expectedRootResponseBody {
						return 0, fmt.Errorf("the message content is %q, but expected %q", string(body), expectedRootResponseBody)
					}
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
				// HTTP should route to HTTPS.
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(strings.Replace(url, "https", "http", 1))
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(fmt.Sprintf("%s/admin", url))
					if fetchErr != nil {
						return 0, fetchErr
					}
					defer resp.Body.Close()
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return 0, err
					}
					if string(body) != expectedAdminResponseBody {
						return 0, fmt.Errorf("the message content is %q, but expected %q", string(body), expectedAdminResponseBody)
					}
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
