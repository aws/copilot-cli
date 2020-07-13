// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("addons flow", func() {
	Context("when creating a new app", func() {
		var (
			initErr error
		)
		BeforeAll(func() {
			_, initErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
		})

		It("app init succeeds", func() {
			Expect(initErr).NotTo(HaveOccurred())
		})

		It("app init creates an copilot directory and workspace file", func() {
			Expect("./copilot").Should(BeADirectory())
			Expect("./copilot/.workspace").Should(BeAnExistingFile())
		})

		It("app ls includes new app", func() {
			apps, err := cli.AppList()
			Expect(err).NotTo(HaveOccurred())
			Expect(apps).To(ContainSubstring(appName))
		})

		It("app show includes app name", func() {
			appShowOutput, err := cli.AppShow(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(appShowOutput.Name).To(Equal(appName))
			Expect(appShowOutput.URI).To(BeEmpty())
		})
	})

	Context("when creating a new environment", func() {
		var (
			testEnvInitErr error
		)
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: "default",
				Prod:    false,
			})
		})

		It("env init should succeed", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when adding a svc", func() {
		var (
			svcInitErr error
		)
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

		It("svc init should create svc manifests", func() {
			Expect("./copilot/hello/manifest.yml").Should(BeAnExistingFile())

		})

		It("svc ls should list the service", func() {
			svcList, svcListError := cli.SvcList(appName)
			Expect(svcListError).NotTo(HaveOccurred())
			Expect(len(svcList.Services)).To(Equal(1))

			svcsByName := map[string]client.SvcDescription{}
			for _, svc := range svcList.Services {
				svcsByName[svc.Name] = svc
			}

			for _, svc := range []string{svcName} {
				Expect(svcsByName[svc].AppName).To(Equal(appName))
				Expect(svcsByName[svc].Name).To(Equal(svc))
			}
		})
	})

	Context("copy addons file to copilot dir", func() {
		It("should copy all addons/ files to the app's workspace", func() {
			err := os.MkdirAll("./copilot/hello/addons", 0777)
			Expect(err).NotTo(HaveOccurred(), "create addons dir")

			fds, err := ioutil.ReadDir("./hello/addons")
			Expect(err).NotTo(HaveOccurred(), "read addons dir")

			for _, fd := range fds {
				destFile, err := os.Create(fmt.Sprintf("./copilot/hello/addons/%s", fd.Name()))
				Expect(err).NotTo(HaveOccurred(), "create destination file")
				defer destFile.Close()

				srcFile, err := os.Open(fmt.Sprintf("./hello/addons/%s", fd.Name()))
				Expect(err).NotTo(HaveOccurred(), "open source file")
				defer srcFile.Close()

				_, err = io.Copy(destFile, srcFile)
				Expect(err).NotTo(HaveOccurred(), "copy file")

			}
		})
	})

	Context("when deploying svc", func() {
		var (
			appDeployErr error
			svcInitErr   error
			initErr      error
		)
		BeforeAll(func() {
			_, appDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     svcName,
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})
		})

		It("svc deploy should succeed", func() {
			Expect(appDeployErr).NotTo(HaveOccurred())
		})

		It("should be able to make a POST request", func() {
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			// Make a POST request to the API to store the user name in DynamoDB.
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal("test"))
			Eventually(func() (int, error) {
				resp, fetchErr := http.Post(fmt.Sprintf("%s/%s/%s", route.URL, svcName, appName), "application/json", nil)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(201))
		})

		It("should be able to retrieve the results", func() {
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			// Make a GET request to the API to retrieve the list of user names from DynamoDB.
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal("test"))
			var resp *http.Response
			var fetchErr error
			Eventually(func() (int, error) {
				resp, fetchErr = http.Get(fmt.Sprintf("%s/hello", route.URL))
				return resp.StatusCode, fetchErr
			}, "10s", "1s").Should(Equal(200))

			bodyBytes, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			type Result struct {
				Names []string
			}
			result := Result{}
			err = json.Unmarshal(bodyBytes, &result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Names[0]).To(Equal(appName))
		})

		It("svc logs should display logs", func() {
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
		})

		It("svc delete should not delete local files", func() {
			_, err := cli.SvcDelete(svcName)
			Expect(err).NotTo(HaveOccurred())
			Expect("./copilot/hello/addons").Should(BeADirectory())
			Expect("./copilot/hello/manifest.yml").Should(BeAnExistingFile())
			Expect("./copilot/.workspace").Should(BeAnExistingFile())

			// Need to recreate the service for AfterSuite testing.
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       svcName,
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./hello/Dockerfile",
				SvcPort:    "80",
			})
			Expect(svcInitErr).NotTo(HaveOccurred())
		})

		It("app delete does remove .workspace but keep local files", func() {
			_, err := cli.AppDelete(map[string]string{"test": "default"})
			Expect(err).NotTo(HaveOccurred())
			Expect("./copilot").Should(BeADirectory())
			Expect("./copilot/hello/addons").Should(BeADirectory())
			Expect("./copilot/hello/manifest.yml").Should(BeAnExistingFile())
			Expect("./copilot/.workspace").ShouldNot(BeAnExistingFile())

			// Need to recreate the app for AfterSuite testing.
			_, initErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
			Expect(initErr).NotTo(HaveOccurred())
		})
	})
})
