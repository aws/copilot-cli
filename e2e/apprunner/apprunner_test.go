// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package apprunner_test

import (
	"fmt"
	"net/http"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type countAssertionTracker struct {
	expected int
	actual   int
}

var _ = Describe("App Runner", func() {

	var (
		initErr error
	)

	BeforeAll(func() {
		_, initErr = cli.Init(&client.InitRequest{
			AppName:      appName,
			WorkloadName: svcName,
			ImageTag:     "gallopinggurdey",
			Dockerfile:   "./front-end/Dockerfile",
			WorkloadType: "Request-Driven Web Service",
			Deploy:       true,
			SvcPort:      "80",
		})
	})

	Context("run init with app runner", func() {
		It("init does not return an error", func() {
			Expect(initErr).NotTo(HaveOccurred())
		})
	})

	Context("run svc ls to ensure the service was created", func() {
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
			Expect(svcList.Services[0].Type).To(Equal("Request-Driven Web Service"))
		})
	})

	Context("run svc status to ensure that the service is healthy", func() {
		var (
			out            *client.SvcStatusOutput
			svcStatusError error
		)

		BeforeAll(func() {
			out, svcStatusError = cli.SvcStatus(&client.SvcStatusRequest{
				Name:    svcName,
				AppName: appName,
				EnvName: envName,
			})
		})

		It("should not return an error", func() {
			Expect(svcStatusError).NotTo(HaveOccurred())
		})

		It("should return app runner service status", func() {
			Expect(out.Status).To(Equal("RUNNING"))
		})
	})

	Context("run storage init and svc deploy to create an S3 bucket", func() {
		var (
			storageInitErr error
			deployErr      error
		)

		BeforeAll(func() {
			_, storageInitErr = cli.StorageInit(&client.StorageInitRequest{
				StorageName:  "s3storage",
				StorageType:  "S3",
				WorkloadName: svcName,
			})
			_, deployErr = cli.SvcDeploy(&client.SvcDeployInput{
				EnvName: envName,
				Name:    svcName,
			})
		})

		It("should not return an error", func() {
			Expect(storageInitErr).NotTo(HaveOccurred())
			Expect(deployErr).NotTo(HaveOccurred())
		})
	})

	Context("run svc show to retrieve the service configuration, resources, and endpoint, then query the service", func() {
		var (
			svc          *client.SvcShowOutput
			svcShowError error
		)

		BeforeAll(func() {
			svc, svcShowError = cli.SvcShow(&client.SvcShowRequest{
				Name:      svcName,
				AppName:   appName,
				Resources: true,
			})
		})

		It("should not return an error", func() {
			Expect(svcShowError).NotTo(HaveOccurred())
		})

		It("should return correct configuration", func() {
			Expect(svc.SvcName).To(Equal(svcName))
			Expect(svc.AppName).To(Equal(appName))
			Expect(len(svc.Configs)).To(Equal(1))
			Expect(svc.Configs[0].Environment).To(Equal(envName))
			Expect(svc.Configs[0].CPU).To(Equal("1024"))
			Expect(svc.Configs[0].Memory).To(Equal("2048"))
			Expect(svc.Configs[0].Port).To(Equal("80"))
		})

		It("should return correct environment variables", func() {
			fmt.Printf("\n\nenvironment variables: %+v\n\n", svc.Variables)
			Expect(len(svc.Variables)).To(Equal(4))
			expectedVars := map[string]string{
				"COPILOT_APPLICATION_NAME": appName,
				"COPILOT_ENVIRONMENT_NAME": envName,
				"COPILOT_SERVICE_NAME":     svcName,
				"S3STORAGE_NAME":           fmt.Sprintf("%s-%s-%s-s3storage", appName, envName, svcName),
			}
			for _, variable := range svc.Variables {
				Expect(variable.Value).To(Equal(expectedVars[variable.Name]))
			}
		})

		It("should return the correct resources", func() {
			Expect(len(svc.Resources)).To(Equal(1))
			Expect(svc.Resources[envName]).NotTo(BeNil())
			Expect(len(svc.Resources[envName])).To(Equal(4))
			expectedTypes := map[string]*countAssertionTracker{
				"AWS::IAM::Role":             {2, 0},
				"AWS::CloudFormation::Stack": {1, 0},
				"AWS::AppRunner::Service":    {1, 0},
			}

			for _, r := range svc.Resources[envName] {
				if expectedTypes[r.Type] == nil {
					expectedTypes[r.Type] = &countAssertionTracker{0, 0}
				}
				expectedTypes[r.Type].actual++
			}

			for t, v := range expectedTypes {
				Expect(v.actual).To(
					Equal(v.expected),
					fmt.Sprintf("Expected %v resources of type %v, received %v", v.expected, t, v.actual))
			}
		})

		It("should return svc endpoints", func() {
			Expect(len(svc.Routes)).To(Equal(1))
			Expect(svc.Routes[0].Environment).To(Equal(envName))
			Expect(svc.Routes[0].URL).NotTo(BeEmpty())
			Eventually(func() (int, error) {
				resp, fetchErr := http.Get(svc.Routes[0].URL)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))
		})
	})

	Context("run svc logs to troubleshoot", func() {
		var (
			svcLogs    []client.SvcLogsOutput
			svcLogsErr error
		)

		BeforeAll(func() {
			svcLogs, svcLogsErr = cli.SvcLogs(&client.SvcLogsRequest{
				AppName: appName,
				Name:    svcName,
				EnvName: "test",
				Since:   "1h",
			})
		})

		It("should not return an error", func() {
			Expect(svcLogsErr).NotTo(HaveOccurred())
		})

		It("should return valid log lines", func() {
			Expect(len(svcLogs)).To(BeNumerically(">", 0))
			for _, logLine := range svcLogs {
				Expect(logLine.Message).NotTo(Equal(""))
				Expect(logLine.LogStreamName).NotTo(Equal(""))
				Expect(logLine.Timestamp).NotTo(Equal(0))
				Expect(logLine.IngestionTime).NotTo(Equal(0))
			}
		})
	})

	Context("run pause and then resume the service", func() {
		var (
			svcPauseError         error
			pausedSvcStatus       *client.SvcStatusOutput
			pausedSvcStatusError  error
			svcResumeError        error
			resumedSvcStatus      *client.SvcStatusOutput
			resumedSvcStatusError error
		)

		BeforeAll(func() {
			_, svcPauseError = cli.SvcPause(&client.SvcPauseRequest{
				AppName: appName,
				EnvName: envName,
				Name:    svcName,
			})
			pausedSvcStatus, pausedSvcStatusError = cli.SvcStatus(&client.SvcStatusRequest{
				Name:    svcName,
				AppName: appName,
				EnvName: envName,
			})
			_, svcResumeError = cli.SvcResume(&client.SvcResumeRequest{
				AppName: appName,
				EnvName: envName,
				Name:    svcName,
			})
			resumedSvcStatus, resumedSvcStatusError = cli.SvcStatus(&client.SvcStatusRequest{
				Name:    svcName,
				AppName: appName,
				EnvName: envName,
			})
		})

		It("should not an return error", func() {
			Expect(svcPauseError).NotTo(HaveOccurred())
			Expect(pausedSvcStatusError).NotTo(HaveOccurred())
			Expect(svcResumeError).NotTo(HaveOccurred())
			Expect(resumedSvcStatusError).NotTo(HaveOccurred())
		})

		It("should successfully pause service", func() {
			Expect(pausedSvcStatus.Status).To(Equal("PAUSED"))
		})

		It("should successfully resume service", func() {
			Expect(resumedSvcStatus.Status).To(Equal("RUNNING"))
		})
	})

})
