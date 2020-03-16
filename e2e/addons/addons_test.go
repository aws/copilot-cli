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

	"github.com/aws/amazon-ecs-cli-v2/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("addons flow", func() {
	Context("when creating a new project", func() {
		var (
			initErr error
		)
		BeforeAll(func() {
			_, initErr = cli.ProjectInit(&client.ProjectInitRequest{
				ProjectName: projectName,
			})
		})

		It("project init succeeds", func() {
			Expect(initErr).NotTo(HaveOccurred())
		})

		It("project init creates an ecs-project directory", func() {
			Expect("./ecs-project").Should(BeADirectory())
		})

		It("project ls includes new project", func() {
			projects, err := cli.ProjectList()
			Expect(err).NotTo(HaveOccurred())
			Expect(projects).To(ContainSubstring(projectName))
		})

		It("project show includes project name", func() {
			projectShowOutput, err := cli.ProjectShow(projectName)
			Expect(err).NotTo(HaveOccurred())
			Expect(projectShowOutput.Name).To(Equal(projectName))
			Expect(projectShowOutput.URI).To(BeEmpty())
		})
	})

	Context("when creating a new environment", func() {
		var (
			testEnvInitErr error
		)
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				ProjectName: projectName,
				EnvName:     "test",
				Profile:     "default",
				Prod:        false,
			})
		})

		It("env init should succeed", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when adding an app", func() {
		var (
			appInitErr error
		)
		BeforeAll(func() {
			_, appInitErr = cli.AppInit(&client.AppInitRequest{
				AppName:    appName,
				AppType:    "Load Balanced Web App",
				Dockerfile: "./hello/Dockerfile",
				AppPort:    "80",
			})
		})

		It("app init should succeed", func() {
			Expect(appInitErr).NotTo(HaveOccurred())
		})

		It("app init should create app manifests", func() {
			Expect("./ecs-project/hello/manifest.yml").Should(BeAnExistingFile())

		})

		It("app ls should list the apps", func() {
			appList, appListError := cli.AppList(projectName)
			Expect(appListError).NotTo(HaveOccurred())
			Expect(len(appList.Apps)).To(Equal(1))

			appsByName := map[string]client.AppDescription{}
			for _, app := range appList.Apps {
				appsByName[app.AppName] = app
			}

			for _, app := range []string{appName} {
				Expect(appsByName[app].AppName).To(Equal(app))
				Expect(appsByName[app].Project).To(Equal(projectName))
			}
		})
	})

	Context("copy addons file to ecs-project", func() {
		It("should copy all addons/ files to the app's workspace", func() {
			err := os.MkdirAll("./ecs-project/hello/addons", 0777)
			Expect(err).NotTo(HaveOccurred(), "create addons dir")

			fds, err := ioutil.ReadDir("./hello/addons")
			Expect(err).NotTo(HaveOccurred(), "read addons dir")

			for _, fd := range fds {
				destFile, err := os.Create(fmt.Sprintf("./ecs-project/hello/addons/%s", fd.Name()))
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

	Context("when deploying app", func() {
		var (
			appDeployErr error
		)
		BeforeAll(func() {
			_, appDeployErr = cli.AppDeploy(&client.AppDeployInput{
				AppName:  appName,
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})
		})

		It("app deploy should succeed", func() {
			Expect(appDeployErr).NotTo(HaveOccurred())
		})

		It("should be able to make a POST request", func() {
			app, appShowErr := cli.AppShow(&client.AppShowRequest{
				ProjectName: projectName,
				AppName:     appName,
			})
			Expect(appShowErr).NotTo(HaveOccurred())
			Expect(len(app.Routes)).To(Equal(1))

			// Make a POST request to the API to store the user name in DynamoDB.
			route := app.Routes[0]
			Expect(route.Environment).To(Equal("test"))
			Eventually(func() (int, error) {
				resp, fetchErr := http.Post(fmt.Sprintf("%s/%s/%s", route.URL, appName, projectName), "application/json", nil)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(201))
		})

		It("should be able to retrieve the results", func() {
			app, appShowErr := cli.AppShow(&client.AppShowRequest{
				ProjectName: projectName,
				AppName:     appName,
			})
			Expect(appShowErr).NotTo(HaveOccurred())
			Expect(len(app.Routes)).To(Equal(1))

			// Make a GET request to the API to retrieve the list of user names from DynamoDB.
			route := app.Routes[0]
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
			Expect(result.Names[0]).To(Equal(projectName))
		})

		It("app logs should display logs", func() {
			var appLogs []client.AppLogsOutput
			var appLogsErr error
			Eventually(func() ([]client.AppLogsOutput, error) {
				appLogs, appLogsErr = cli.AppLogs(&client.AppLogsRequest{
					ProjectName: projectName,
					AppName:     appName,
					EnvName:     "test",
					Since:       "1h",
				})
				return appLogs, appLogsErr
			}, "60s", "10s").ShouldNot(BeEmpty())

			for _, logLine := range appLogs {
				Expect(logLine.Message).NotTo(Equal(""))
				Expect(logLine.TaskID).NotTo(Equal(""))
				Expect(logLine.Timestamp).NotTo(Equal(0))
				Expect(logLine.IngestionTime).NotTo(Equal(0))
			}
		})
	})
})
