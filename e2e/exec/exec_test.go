// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("exec flow", func() {
	Context("when creating a new app", Ordered, func() {
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
			Eventually(cli.AppList, "30s", "5s").Should(ContainSubstring(appName))
		})

		It("app show includes app name", func() {
			appShowOutput, err := cli.AppShow(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(appShowOutput.Name).To(Equal(appName))
			Expect(appShowOutput.URI).To(BeEmpty())
		})
	})

	Context("when adding a new environment", Ordered, func() {
		var (
			testEnvInitErr error
		)
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: envName,
				Profile: envName,
			})
		})

		It("env init should succeed", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environment", Ordered, func() {
		var envDeployErr error
		BeforeAll(func() {
			_, envDeployErr = cli.EnvDeploy(&client.EnvDeployRequest{
				AppName: appName,
				Name:    envName,
			})
		})

		It("should succeed", func() {
			Expect(envDeployErr).NotTo(HaveOccurred())
		})
	})

	Context("when adding a svc", Ordered, func() {
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

			svcsByName := map[string]client.WkldDescription{}
			for _, svc := range svcList.Services {
				svcsByName[svc.Name] = svc
			}

			for _, svc := range []string{svcName} {
				Expect(svcsByName[svc].AppName).To(Equal(appName))
				Expect(svcsByName[svc].Name).To(Equal(svc))
			}
		})
	})

	Context("when deploying svc", Ordered, func() {
		const newContent = "HELP I AM TRAPPED INSIDE A SHELL"
		var (
			appDeployErr error
			taskID1      string
			taskID2      string
		)
		BeforeAll(func() {
			_, appDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     svcName,
				EnvName:  envName,
				ImageTag: "gallopinggurdey",
			})
		})

		It("svc deploy should succeed", func() {
			Expect(appDeployErr).NotTo(HaveOccurred())
		})

		It("svc status should show two tasks running", func() {
			svc, svcStatusErr := cli.SvcStatus(&client.SvcStatusRequest{
				AppName: appName,
				Name:    svcName,
				EnvName: envName,
			})
			Expect(svcStatusErr).NotTo(HaveOccurred())
			// Service should be active.
			Expect(svc.Service.Status).To(Equal("ACTIVE"))
			// Should have correct number of running tasks.
			Expect(len(svc.Tasks)).To(Equal(2))
			taskID1 = svc.Tasks[0].ID
			taskID2 = svc.Tasks[1].ID
		})

		It("svc show should include a valid URL and description for test env", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))
			Expect(len(svc.ServiceConnects)).To(Equal(0))

			route := svc.Routes[0]
			Expect(route.Environment).To(Equal(envName))
			var resp *http.Response
			var fetchErr error
			Eventually(func() (int, error) {
				resp, fetchErr = http.Get(route.URL)
				return resp.StatusCode, fetchErr
			}, "60s", "1s").Should(Equal(200))
			// Read the response - our deployed service should return a body with its
			// name as the value.
			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(bodyBytes)).To(Equal(svcName))
		})

		It("session manager should be installed", func() {
			// Use custom SSM plugin as the public version is not compatible to Alpine Linux.
			err := client.BashExec("chmod +x ./session-manager-plugin")
			Expect(err).NotTo(HaveOccurred())
			err = client.BashExec("mv ./session-manager-plugin /bin/session-manager-plugin")
			Expect(err).NotTo(HaveOccurred())
		})

		It("svc exec should be able to modify the content of the website", func() {
			_, err := cli.SvcExec(&client.SvcExecRequest{
				Name:    svcName,
				AppName: appName,
				TaskID:  taskID1,
				Command: fmt.Sprintf(`/bin/sh -c "echo '%s' > /usr/share/nginx/html/index.html"`, newContent),
				EnvName: envName,
			})
			Expect(err).NotTo(HaveOccurred())
			_, err = cli.SvcExec(&client.SvcExecRequest{
				Name:    svcName,
				AppName: appName,
				TaskID:  taskID2,
				Command: fmt.Sprintf(`/bin/sh -c "echo '%s' > /usr/share/nginx/html/index.html"`, newContent),
				EnvName: envName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("website content should be modified", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))
			route := svc.Routes[0]

			for i := 0; i < 5; i++ {
				var resp *http.Response
				var fetchErr error
				Eventually(func() (int, error) {
					resp, fetchErr = http.Get(route.URL)
					return resp.StatusCode, fetchErr
				}, "60s", "1s").Should(Equal(200))
				// Our deployed service should return a body with the new content
				// as the value.
				bodyBytes, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(string(bodyBytes))).To(Equal(newContent))
				time.Sleep(3 * time.Second)
			}
		})

		// It("svc logs should include exec logs", func() {
		// 	var validTaskExecLogsCount int
		// 	for i := 0; i < 10; i++ {
		// 		var svcLogs []client.SvcLogsOutput
		// 		var svcLogsErr error
		// 		Eventually(func() ([]client.SvcLogsOutput, error) {
		// 			svcLogs, svcLogsErr = cli.SvcLogs(&client.SvcLogsRequest{
		// 				AppName: appName,
		// 				Name:    svcName,
		// 				EnvName: envName,
		// 				Since:   "1m",
		// 			})
		// 			return svcLogs, svcLogsErr
		// 		}, "60s", "10s").ShouldNot(BeEmpty())
		// 		var prevExecLogStreamName string
		// 		for _, logLine := range svcLogs {
		// 			Expect(logLine.Message).NotTo(Equal(""))
		// 			Expect(logLine.LogStreamName).NotTo(Equal(""))
		// 			Expect(logLine.Timestamp).NotTo(Equal(0))
		// 			Expect(logLine.IngestionTime).NotTo(Equal(0))
		// 			if strings.Contains(logLine.LogStreamName, "ecs-execute-command") &&
		// 				logLine.LogStreamName != prevExecLogStreamName {
		// 				validTaskExecLogsCount++
		// 				prevExecLogStreamName = logLine.LogStreamName
		// 			}
		// 		}
		// 		if validTaskExecLogsCount == 2 {
		// 			break
		// 		}
		// 		validTaskExecLogsCount = 0
		// 		time.Sleep(5 * time.Second)
		// 	}
		// 	Expect(validTaskExecLogsCount).To(Equal(2))
		// })
	})

	Context("when running a one-off task", Ordered, func() {
		var (
			taskRunErr error
		)
		BeforeAll(func() {
			_, taskRunErr = cli.TaskRun(&client.TaskRunInput{
				GroupName:  groupName,
				Dockerfile: "./DockerfileTask",
				AppName:    appName,
				Env:        envName,
			})
		})

		It("should succeed", func() {
			Expect(taskRunErr).NotTo(HaveOccurred())
		})

		It("task exec should work", func() {
			var resp string
			var err error
			Eventually(func() (string, error) {
				resp, err = cli.TaskExec(&client.TaskExecRequest{
					Name:    groupName,
					AppName: appName,
					Command: "ls",
					EnvName: envName,
				})
				return resp, err
			}, "120s", "20s").Should(ContainSubstring("hello"))
		})
	})
})
