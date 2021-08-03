// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package init_test

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("init flow", func() {

	var (
		svcName    string
		jobName    string
		initErr    error
		jobInitErr error
	)

	BeforeAll(func() {
		svcName = "front-end"
		_, initErr = cli.Init(&client.InitRequest{
			AppName:      appName,
			WorkloadName: svcName,
			ImageTag:     "gallopinggurdey",
			Dockerfile:   "./front-end/Dockerfile",
			WorkloadType: "Load Balanced Web Service",
			Deploy:       true,
			SvcPort:      "80",
		})
		jobName = "mailer"
		_, jobInitErr = cli.Init(&client.InitRequest{
			AppName:      appName,
			WorkloadName: jobName,
			ImageTag:     "thepostalservice",
			Dockerfile:   "./mailer/Dockerfile",
			WorkloadType: "Scheduled Job",
			Deploy:       true,
			Schedule:     "@every 5m",
		})
	})

	Context("creating a brand new app, svc, job, and deploying to a test environment", func() {
		It("init does not return an error", func() {
			Expect(initErr).NotTo(HaveOccurred())
		})

		It("init with job does not return an error", func() {
			Expect(jobInitErr).NotTo(HaveOccurred())
		})
	})

	Context("svc ls", func() {
		var (
			svcList      *client.SvcListOutput
			svcListError error
		)

		BeforeAll(func() {
			svcList, svcListError = cli.SvcList(appName)
		})

		It("should not return an error", func() {
			Expect(svcListError).NotTo(HaveOccurred())
		})

		It("should return one service", func() {
			Expect(len(svcList.Services)).To(Equal(1))
			Expect(svcList.Services[0].Name).To(Equal(svcName))
			Expect(svcList.Services[0].AppName).To(Equal(appName))
		})

	})

	Context("job ls", func() {
		var (
			jobList    *client.JobListOutput
			jobListErr error
		)

		BeforeAll(func() {
			jobList, jobListErr = cli.JobList(appName)
		})

		It("should not return a job list error", func() {
			Expect(jobListErr).NotTo(HaveOccurred())
		})

		It("should return one job", func() {
			Expect(len(jobList.Jobs)).To(Equal(1))
			Expect(jobList.Jobs[0].Name).To(Equal(jobName))
			Expect(jobList.Jobs[0].AppName).To(Equal(appName))
		})
	})

	Context("svc show", func() {
		var (
			svc        *client.SvcShowOutput
			svcShowErr error
		)

		BeforeAll(func() {
			svc, svcShowErr = cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})

		})

		It("should not return an error", func() {
			Expect(svcShowErr).NotTo(HaveOccurred())
		})

		It("should return the correct configuration", func() {
			Expect(svc.SvcName).To(Equal(svcName))
			Expect(svc.AppName).To(Equal(appName))
		})

		It("should return a valid route", func() {
			Expect(len(svc.Routes)).To(Equal(1))
			Expect(svc.Routes[0].Environment).To(Equal("test"))
			Eventually(func() (int, error) {
				resp, fetchErr := http.Get(svc.Routes[0].URL)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))
		})

		It("should return a valid service discovery namespace", func() {
			Expect(len(svc.ServiceDiscoveries)).To(Equal(1))
			Expect(svc.ServiceDiscoveries[0].Environment).To(Equal([]string{"test"}))
			Expect(svc.ServiceDiscoveries[0].Namespace).To(Equal(fmt.Sprintf("%s.%s.%s.local:80", svcName, envName, appName)))
		})

		It("should return the correct environment variables", func() {
			Expect(len(svc.Variables)).To(Equal(5))
			expectedVars := map[string]string{
				"COPILOT_APPLICATION_NAME":           appName,
				"COPILOT_ENVIRONMENT_NAME":           "test",
				"COPILOT_LB_DNS":                     strings.TrimPrefix(svc.Routes[0].URL, "http://"),
				"COPILOT_SERVICE_NAME":               svcName,
				"COPILOT_SERVICE_DISCOVERY_ENDPOINT": fmt.Sprintf("%s.%s.local", envName, appName),
			}
			for _, variable := range svc.Variables {
				Expect(variable.Value).To(Equal(expectedVars[variable.Name]))
			}
		})
	})

	Context("svc logs", func() {

		It("should return valid log lines", func() {
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
	})
})
