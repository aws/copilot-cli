// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package multi_svc_app_test

import (
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

var (
	initErr error
)

var _ = Describe("Multiple Service App", func() {
	Context("when creating a new app", func() {
		BeforeAll(func() {
			_, initErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
		})

		It("app init succeeds", func() {
			Expect(initErr).NotTo(HaveOccurred())
		})

		It("app init creates a copilot directory", func() {
			Expect("./copilot").Should(BeADirectory())
		})

		It("app ls includes new application", func() {
			Eventually(cli.AppList, "30s", "5s").Should(ContainSubstring(appName))
		})

		It("app show includes app name", func() {
			appShowOutput, err := cli.AppShow(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(appShowOutput.Name).To(Equal(appName))
			Expect(appShowOutput.URI).To(BeEmpty())
		})
	})

	Context("when adding a new environment", func() {
		var (
			testEnvInitErr error
		)
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: "default",
			})
		})

		It("env init should succeed", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environment", func() {
		var envDeployErr error
		BeforeAll(func() {
			_, envDeployErr = cli.EnvDeploy(&client.EnvDeployRequest{
				AppName: appName,
				Name:    "test",
			})
		})

		It("should succeed", func() {
			Expect(envDeployErr).NotTo(HaveOccurred())
		})
	})

	Context("when adding a svc", func() {
		var (
			frontEndInitErr error
			wwwInitErr      error
			backEndInitErr  error
			jobInitErr      error
		)
		BeforeAll(func() {

			_, frontEndInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "front-end",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./front-end/Dockerfile",
			})
			_, wwwInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "www",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./www/Dockerfile",
				SvcPort:    "80",
			})

			_, backEndInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "back-end",
				SvcType:    "Backend Service",
				Dockerfile: "./back-end/Dockerfile",
				SvcPort:    "80",
			})

			_, jobInitErr = cli.JobInit(&client.JobInitInput{
				Name:       "query",
				Dockerfile: "./query/Dockerfile",
				Schedule:   "@every 4m",
			})
		})

		It("svc init should succeed", func() {
			Expect(frontEndInitErr).NotTo(HaveOccurred())
			Expect(wwwInitErr).NotTo(HaveOccurred())
			Expect(backEndInitErr).NotTo(HaveOccurred())
		})

		It("job init should succeed", func() {
			Expect(jobInitErr).NotTo(HaveOccurred())
		})

		It("svc init should create svc manifests", func() {
			Expect("./copilot/front-end/manifest.yml").Should(BeAnExistingFile())
			Expect("./copilot/www/manifest.yml").Should(BeAnExistingFile())
			Expect("./copilot/back-end/manifest.yml").Should(BeAnExistingFile())
		})

		It("job init should create job manifest", func() {
			Expect("./copilot/query/manifest.yml").Should(BeAnExistingFile())
		})

		It("svc ls should list the svc", func() {
			svcList, svcListError := cli.SvcList(appName)
			Expect(svcListError).NotTo(HaveOccurred())
			Expect(len(svcList.Services)).To(Equal(3))

			svcsByName := map[string]client.WkldDescription{}
			for _, svc := range svcList.Services {
				svcsByName[svc.Name] = svc
			}

			for _, svc := range []string{"front-end", "www", "back-end"} {
				Expect(svcsByName[svc].Name).To(Equal(svc))
				Expect(svcsByName[svc].AppName).To(Equal(appName))
			}
		})

		It("job ls should list the job", func() {
			jobList, jobListError := cli.JobList(appName)
			Expect(jobListError).NotTo(HaveOccurred())
			Expect(len(jobList.Jobs)).To(Equal(1))

			jobsByName := map[string]client.WkldDescription{}
			for _, job := range jobList.Jobs {
				jobsByName[job.Name] = job
			}

			Expect(jobsByName["query"].Name).To(Equal("query"))
			Expect(jobsByName["query"].AppName).To(Equal(appName))
		})

		It("svc package should output a cloudformation template and params file", func() {
			_, svcPackageError := cli.SvcPackage(&client.PackageInput{
				Name:    "front-end",
				AppName: appName,
				Env:     "test",
				Dir:     "infrastructure",
				Tag:     "gallopinggurdey",
			})
			Expect(svcPackageError).NotTo(HaveOccurred())
			Expect("infrastructure/front-end-test.stack.yml").To(BeAnExistingFile())
			Expect("infrastructure/front-end-test.params.json").To(BeAnExistingFile())
		})

		It("job package should output a Cloudformation template and params file", func() {
			_, jobPackageError := cli.JobPackage(&client.PackageInput{
				Name:    "query",
				AppName: appName,
				Env:     "test",
				Dir:     "infrastructure",
				Tag:     "thepostalservice",
			})
			Expect(jobPackageError).NotTo(HaveOccurred())
			Expect("infrastructure/query-test.params.json").To(BeAnExistingFile())
			Expect("infrastructure/query-test.stack.yml").To(BeAnExistingFile())
		})
	})

	Context("when deploying services and jobs", func() {
		var (
			frontEndDeployErr error
			wwwDeployErr      error
			backEndDeployErr  error
			jobDeployErr      error

			routeURL string
		)
		BeforeAll(func() {
			_, frontEndDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     "front-end",
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})
			_, jobDeployErr = cli.JobDeploy(&client.JobDeployInput{
				Name:     "query",
				EnvName:  "test",
				ImageTag: "thepostalservice",
			})
			_, wwwDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     "www",
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})
			_, backEndDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     "back-end",
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})
		})

		It("svc deploy should succeed", func() {
			Expect(frontEndDeployErr).NotTo(HaveOccurred())
			Expect(wwwDeployErr).NotTo(HaveOccurred())
			Expect(backEndDeployErr).NotTo(HaveOccurred())
		})

		It("job deploy should succeed", func() {
			Expect(jobDeployErr).NotTo(HaveOccurred())
		})

		It("svc show should include a valid URL and description for test env", func() {
			for _, svcName := range []string{"front-end", "www"} {
				svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
					AppName: appName,
					Name:    svcName,
				})
				Expect(svcShowErr).NotTo(HaveOccurred())
				Expect(len(svc.Routes)).To(Equal(1))

				// Call each environment's endpoint and ensure it returns a 200
				route := svc.Routes[0]
				Expect(route.Environment).To(Equal("test"))
				// Since the front-end was added first, it should have no suffix.
				if svcName == "front-end" {
					Expect(route.URL).ToNot(HaveSuffix(svcName))
				}

				// Since the www app was added second, it should have app appended to the name.
				var resp *http.Response
				var fetchErr error
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(route.URL)
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))

				// Read the response - our deployed apps should return a body with their
				// name as the value.
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(bodyBytes)).To(Equal(svcName))
			}
		})

		It("svc status should include the service, tasks, and alarm status", func() {
			svcName := "front-end"
			svc, svcStatusErr := cli.SvcStatus(&client.SvcStatusRequest{
				AppName: appName,
				Name:    svcName,
				EnvName: "test",
			})
			Expect(svcStatusErr).NotTo(HaveOccurred())
			// Service should be active.
			Expect(svc.Service.Status).To(Equal("ACTIVE"))
			// Desired count should be minimum auto scaling number.
			Expect(svc.Service.DesiredCount).To(Equal(int64(2)))
			// Should have correct number of running tasks.
			Expect(len(svc.Tasks)).To(Equal(2))
			// Should have correct number of auto scaling alarms.
			Expect(len(svc.Alarms)).To(Equal(4))
		})

		It("env show should include the name and type for front-end, www, and back-end svcs", func() {
			envShowOutput, envShowErr := cli.EnvShow(&client.EnvShowRequest{
				AppName: appName,
				EnvName: "test",
			})
			Expect(envShowErr).NotTo(HaveOccurred())
			Expect(len(envShowOutput.Services)).To(Equal(3))
			svcs := map[string]client.EnvShowServices{}
			for _, svc := range envShowOutput.Services {
				svcs[svc.Name] = svc
			}
			Expect(svcs["front-end"]).NotTo(BeNil())
			Expect(svcs["front-end"].Type).To(Equal("Load Balanced Web Service"))
			Expect(svcs["www"]).NotTo(BeNil())
			Expect(svcs["www"].Type).To(Equal("Load Balanced Web Service"))
			Expect(svcs["back-end"]).NotTo(BeNil())
			Expect(svcs["back-end"].Type).To(Equal("Backend Service"))
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
			svcName := "front-end"
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			// Calls the front end's service discovery endpoint - which should connect
			// to the backend, and pipe the backend response to us.
			route := svc.Routes[0]

			Expect(route.Environment).To(Equal("test"))
			routeURL = route.URL

			resp, fetchErr := http.Get(fmt.Sprintf("%s/service-discovery-test/", route.URL))
			Expect(fetchErr).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			// Read the response - our deployed apps should return a body with their
			// name as the value.
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(bodyBytes)).To(Equal("back-end-service-discovery"))

		})

		It("should be able to write to EFS volume", func() {
			svcName := "front-end"
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			// Calls the front end's EFS test endpoint - which should create a file in the EFS filesystem.
			route := svc.Routes[0]

			Expect(route.Environment).To(Equal("test"))
			routeURL = route.URL

			resp, fetchErr := http.Get(fmt.Sprintf("%s/efs-putter", route.URL))
			Expect(fetchErr).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))
		})

		It("EFS volume should appear in `env show`", func() {
			envShowOutput, envShowErr := cli.EnvShow(&client.EnvShowRequest{
				AppName: appName,
				EnvName: "test",
			})
			Expect(envShowErr).NotTo(HaveOccurred())
			Expect(envShowOutput.Resources).To(ContainElement(HaveKeyWithValue("type", "AWS::EFS::FileSystem")))
		})

		It("job should have run", func() {
			// Job should have run. We check this by hitting the "job-checker" path, which tells us the value
			// of the "TEST_JOB_CHECK_VAR" in the frontend service, which will have been updated by a GET on
			// /job-setter
			Eventually(func() (string, error) {
				resp, fetchErr := http.Get(fmt.Sprintf("%s/job-checker/", routeURL))
				if fetchErr != nil {
					return "", fetchErr
				}
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					return "", err
				}
				return string(bodyBytes), nil
			}, "4m", "10s").Should(Equal("yes")) // This is shorthand for "error is nil and resp is yes"
		})

		It("environment variable should be overridden and accessible through GET /magicwords", func() {
			// The front-end service has a route called "/magicwords/" which returns the value of
			// an environment variable set by a docker argument. If the argument is not overridden
			// at build time, the endpoint will return "open caraway" in the body. If the value
			// is overridden by the extended build configuration in the manifest, it will return
			// "open sesame" in the body.
			svcName := "front-end"
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			// Calls the front end's magicwords endpoint
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal("test"))
			resp, fetchErr := http.Get(fmt.Sprintf("%s/magicwords/", route.URL))
			Expect(fetchErr).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			// Read the response - successfully overridden build arg will result
			// in a response of "open sesame"
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(bodyBytes)).To(Equal("open sesame"))
		})

		It("svc logs should display logs", func() {
			for _, svcName := range []string{"front-end", "back-end"} {
				var svcLogs []client.SvcLogsOutput
				var svcLogsErr error
				Eventually(func() ([]client.SvcLogsOutput, error) {
					svcLogs, svcLogsErr = cli.SvcLogs(&client.SvcLogsRequest{
						AppName: appName,
						Name:    svcName,
						EnvName: "test",
						Since:   "1h",
					})
					return svcLogs, svcLogsErr
				}, "60s", "10s").ShouldNot(BeEmpty())

				for _, logLine := range svcLogs {
					Expect(logLine.Message).NotTo(Equal(""))
					Expect(logLine.LogStreamName).NotTo(Equal(""))
					Expect(logLine.Timestamp).NotTo(Equal(0))
					Expect(logLine.IngestionTime).NotTo(Equal(0))
				}
			}
		})
	})
})
