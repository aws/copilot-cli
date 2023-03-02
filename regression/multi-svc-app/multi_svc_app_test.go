// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package multi_svc_app_test

import (
	"fmt"
	"net/http"
	"os"
	"io"
	"path/filepath"

	"github.com/aws/copilot-cli/regression/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("regression", func() {
	var cli *client.CLI
	var expectedWorkloadResponse = make(map[string]string)

	deployWorkloadSpecs := func() {
		var routeURLs = make(map[string]string)
		It("workload deploy should succeed", func() {
			for _, svcName := range []string{"front-end", "back-end", "www"} {
				_, err := cli.Run("svc", "deploy",
					"--name", svcName,
					"--env", "test",
				)
				Expect(err).NotTo(HaveOccurred())
			}
			_, jobDeployErr := cli.Run("job", "deploy",
				"--name", "query",
				"--env", "test",
			)
			Expect(jobDeployErr).NotTo(HaveOccurred())
		})

		It("load-balanced web services should be able to make a GET request", func() {
			for _, svcName := range []string{"front-end", "www"} {
				out, err := cli.Run("svc", "show",
					"--app", appName,
					"--name", svcName,
					"--json")
				Expect(err).NotTo(HaveOccurred())
				svc, err := client.ToSvcShowOutput(out)
				Expect(err).NotTo(HaveOccurred())

				By("Having the correct number of routes in svc show")
				Expect(len(svc.Routes)).To(Equal(1))
				route := svc.Routes[0]

				By("Having the correct environment associated with the route")
				Expect(route.Environment).To(Equal("test"))
				routeURLs[svcName] = route.URL

				By("Being able to make a GET request to the API")
				var resp *http.Response
				var fetchErr error
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(routeURLs[svcName])
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))

				By("Having their names as the value in response body")
				bodyBytes, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(bodyBytes)).To(Equal(expectedWorkloadResponse[svcName]))
			}

		})

		It("service discovery should be enabled and working", func() {
			// The front-end service is set up to have a path called
			// "/front-end/service-discovery-test" - this route
			// calls a function which makes a call via the service
			// discovery endpoint, "back-end.local". If that back-end
			// call succeeds, the back-end returns a response
			// "back-end-service-discovery". This should be forwarded
			// back to us via the front-end api.
			// [test] -- http req -> [front-end] -- service-discovery -> [back-end]
			By("Being able to call the service discovery endpoint from front-end")
			url := routeURLs["front-end"]
			resp, fetchErr := http.Get(fmt.Sprintf("%s/service-discovery-test/", url))
			Expect(fetchErr).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			By("Getting the expected response body")
			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(bodyBytes)).To(Equal(expectedWorkloadResponse["back-end-service-discovery"]))

		})

		It("job should have run", func() {
			// Job should have run. We check this by hitting the "job-checker" path, which tells us the value
			// of the "TEST_JOB_CHECK_VAR" in the frontend service, which will have been updated by a GET on
			// /job-setter
			Eventually(func() (string, error) {
				resp, fetchErr := http.Get(fmt.Sprintf("%s/job-checker/", routeURLs["front-end"]))
				if fetchErr != nil {
					return "", fetchErr
				}
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					return "", err
				}
				return string(bodyBytes), nil
			}, "4m", "10s").Should(Equal("yes")) // This is shorthand for "error is nil and resp is yes"
		})
	}

	deployEnvironmentSpecs := func() {
		It("should succeed", func() {
			_, envDeployErr := cli.Run("env", "deploy",
				"--name", "test",
				"--app", appName,
				"--force",
			)
			Expect(envDeployErr).NotTo(HaveOccurred())
		})
	}

	When("Using the old CLI", func() {
		BeforeEach(func() {
			cli = fromCLI
			expectedWorkloadResponse = map[string]string{
				"front-end":                  "front-end",
				"www":                        "www",
				"back-end-service-discovery": "back-end-service-discovery",
			}
		})

		When("Creating a new app", func() {
			It("app init succeeds", func() {
				_, initErr := cli.Run("app", "init", appName)
				Expect(initErr).NotTo(HaveOccurred())
			})

			It("app init creates an copilot directory and workspace file", func() {
				Expect("./copilot").Should(BeADirectory())
				Expect("./copilot/.workspace").Should(BeAnExistingFile())
			})

			It("app ls includes new app", func() {
				Eventually(func() (string, error) {
					return cli.Run("app", "ls")
				}, "30s", "50s").Should(ContainSubstring(appName))
			})
		})

		When("Adding a new environment", func() {
			It("should succeed", func() {
				_, testEnvInitErr := cli.Run("env", "init",
					"--name", "test",
					"--app", appName,
					"--profile", "default",
					"--default-config",
				)
				Expect(testEnvInitErr).NotTo(HaveOccurred())
			})
		})

		When("Deploying a new environment", deployEnvironmentSpecs)

		When("Adding workloads", func() {
			It("workload init should succeed", func() {
				_, err := cli.Run("svc", "init",
					"--name", "front-end",
					"--svc-type", "Load Balanced Web Service",
					"--dockerfile", fmt.Sprintf("./%s/Dockerfile", "front-end"))
				Expect(err).NotTo(HaveOccurred())
				_, err = cli.Run("svc", "init",
					"--name", "www",
					"--svc-type", "Load Balanced Web Service",
					"--port", "80",
					"--dockerfile", fmt.Sprintf("./%s/Dockerfile", "www"))
				Expect(err).NotTo(HaveOccurred())
				_, err = cli.Run("svc", "init",
					"--name", "back-end",
					"--svc-type", "Backend Service",
					"--port", "80",
					"--dockerfile", fmt.Sprintf("./%s/Dockerfile", "back-end"))
				Expect(err).NotTo(HaveOccurred())
				_, err = cli.Run("job", "init",
					"--name", "query",
					"--schedule", "@every 1m",
					"--dockerfile", fmt.Sprintf("./%s/Dockerfile", "query"))
				Expect(err).NotTo(HaveOccurred())
			})

			It("workload init should create manifests", func() {
				Expect("./copilot/front-end/manifest.yml").Should(BeAnExistingFile())
				Expect("./copilot/www/manifest.yml").Should(BeAnExistingFile())
				Expect("./copilot/back-end/manifest.yml").Should(BeAnExistingFile())
				Expect("./copilot/query/manifest.yml").Should(BeAnExistingFile())
			})

			It("workload ls should list the service", func() {
				out, err := cli.Run("svc", "ls",
					"--app", appName,
					"--json")
				Expect(err).NotTo(HaveOccurred())
				svcList, err := client.ToSvcListOutput(out)
				Expect(err).NotTo(HaveOccurred())

				out, err = cli.Run("job", "ls",
					"--app", appName,
					"--json")
				Expect(err).NotTo(HaveOccurred())
				jobList, err := client.ToJobListOutput(out)
				Expect(err).NotTo(HaveOccurred())

				By("Having an expected number of services or jobs")
				Expect(len(svcList.Services)).To(Equal(3))
				Expect(len(jobList.Jobs)).To(Equal(1))
			})
		})

		When("Deploying workloads", deployWorkloadSpecs)
	})

	When("Updating application code", func() {
		It("should succeed", func() {
			files := make(map[string]string)
			for _, svcName := range []string{"front-end", "www", "back-end"} {
				files[filepath.Join(svcName, "main.go")] = filepath.Join(svcName, "swap", "main.go")
			}
			files[filepath.Join("query", "entrypoint.sh")] = filepath.Join("query", "swap", "entrypoint.sh")
			err := swapFiles(files)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("Using the new CLI", func() {
		BeforeEach(func() {
			cli = toCLI
			expectedWorkloadResponse = map[string]string{
				"front-end":                  "front-end oraoraora",
				"www":                        "www oraoraora",
				"back-end-service-discovery": "back-end-service-discovery oraoraora",
			}
		})

		When("Deploying workloads", deployWorkloadSpecs)

		When("Deploying the environment", deployEnvironmentSpecs)
	})

	When("Swapping back the application code", func() {
		It("should succeed", func() {
			files := make(map[string]string)
			for _, svcName := range []string{"front-end", "www", "back-end"} {
				files[filepath.Join(svcName, "swap", "main.go")] = filepath.Join(svcName, "main.go")
			}
			files[filepath.Join("query", "swap", "entrypoint.sh")] = filepath.Join("query", "entrypoint.sh")
			_ = swapFiles(files) // Best-effort: do not consider the test suite fails if an error occurs.
		})
	})
})

func swapFiles(files map[string]string) error {
	for fileA, fileB := range files {
		if err := os.Rename(fileA, fmt.Sprintf("%s.tmp", fileA)); err != nil {
			return err
		}
		if err := os.Rename(fileB, fileA); err != nil {
			return err
		}
		if err := os.Rename(fmt.Sprintf("%s.tmp", fileA), fileB); err != nil {
			return err
		}
	}
	return nil
}
