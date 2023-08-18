// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package apprunner_test

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type countAssertionTracker struct {
	expected int
	actual   int
}

var _ = Describe("App Runner", Ordered, func() {

	var (
		initErr error
	)

	BeforeAll(func() {
		_, initErr = cli.Init(&client.InitRequest{
			AppName:      appName,
			EnvName:      envName,
			WorkloadName: feSvcName,
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
			Expect(svcList.Services[0].Name).To(Equal(feSvcName))
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
				Name:    feSvcName,
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

	Context("create and deploy a backend service", func() {
		var (
			initErr   error
			deployErr error
		)

		BeforeAll(func() {
			_, initErr = cli.SvcInit(&client.SvcInitRequest{
				Name:        beSvcName,
				SvcType:     "Request-Driven Web Service",
				Dockerfile:  "./back-end/Dockerfile",
				SvcPort:     "80",
				IngressType: "Environment",
			})
			_, deployErr = cli.SvcDeploy(&client.SvcDeployInput{
				EnvName: envName,
				Name:    beSvcName,
			})
		})

		It("should not return an error", func() {
			Expect(initErr).NotTo(HaveOccurred())
			Expect(deployErr).NotTo(HaveOccurred())
		})
	})

	Context("run svc show to retrieve the service configuration, resources, secrets, and endpoint, then query the service", func() {
		var (
			svc            *client.SvcShowOutput
			svcShowError   error
			backendSvc     *client.SvcShowOutput
			backendShowErr error
		)

		BeforeAll(func() {
			svc, svcShowError = cli.SvcShow(&client.SvcShowRequest{
				Name:      feSvcName,
				AppName:   appName,
				Resources: true,
			})
			backendSvc, backendShowErr = cli.SvcShow(&client.SvcShowRequest{
				Name:      beSvcName,
				AppName:   appName,
				Resources: true,
			})
		})

		It("should not return an error", func() {
			Expect(svcShowError).NotTo(HaveOccurred())
			Expect(backendShowErr).NotTo(HaveOccurred())
		})

		It("should return correct configuration", func() {
			Expect(svc.SvcName).To(Equal(feSvcName))
			Expect(svc.AppName).To(Equal(appName))
			Expect(len(svc.Configs)).To(Equal(1))
			Expect(svc.Configs[0].Environment).To(Equal(envName))
			Expect(svc.Configs[0].CPU).To(Equal("1024"))
			Expect(svc.Configs[0].Memory).To(Equal("2048"))
			Expect(svc.Configs[0].Port).To(Equal("80"))

			Expect(backendSvc.SvcName).To(Equal(beSvcName))
			Expect(backendSvc.AppName).To(Equal(appName))
			Expect(backendSvc.Configs[0].Environment).To(Equal(envName))
		})

		It("should return correct environment variables", func() {
			fmt.Printf("\n\nenvironment variables: %+v\n\n", svc.Variables)
			expectedVars := map[string]string{
				"COPILOT_APPLICATION_NAME":           appName,
				"COPILOT_ENVIRONMENT_NAME":           envName,
				"COPILOT_SERVICE_NAME":               feSvcName,
				"COPILOT_SERVICE_DISCOVERY_ENDPOINT": fmt.Sprintf("%s.%s.local", envName, appName),
			}
			for _, variable := range svc.Variables {
				Expect(variable.Value).To(Equal(expectedVars[variable.Name]))
			}
		})

		It("should return correct secrets", func() {
			fmt.Printf("\n\nsecrets: %+v\n\n", svc.Secrets)
			for _, envVar := range svc.Secrets {
				if envVar.Name == "my-ssm-param" {
					Expect(envVar.Value).To(Equal(ssmName))
				}
				if envVar.Name == "USER_CREDS" {
					valueFromARN := envVar.Value // E.g. arn:aws:secretsmanager:ap-northeast-1:1111111111:secret:e2e-apprunner-my-secret
					Expect(strings.Contains(valueFromARN, secretName)).To(BeTrue())
				}
			}
		})

		It("should return the correct resources", func() {
			Expect(len(svc.Resources)).To(Equal(1))
			Expect(svc.Resources[envName]).NotTo(BeNil())
			expectedTypes := map[string]*countAssertionTracker{
				"AWS::IAM::Role":                 {3, 0},
				"AWS::AppRunner::Service":        {1, 0},
				"AWS::EC2::SecurityGroup":        {1, 0},
				"AWS::EC2::SecurityGroupIngress": {1, 0},
				"AWS::AppRunner::VpcConnector":   {1, 0},
				"Custom::EnvControllerFunction":  {1, 0},
				"AWS::Lambda::Function":          {1, 0},
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

		It("should return svc endpoints and service discovery should work", func() {
			Expect(len(svc.Routes)).To(Equal(1))
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal(envName))
			Expect(route.URL).NotTo(BeEmpty())
			Expect(route.Ingress).To(Equal("internet"))

			beRoute := backendSvc.Routes[0]
			Expect(beRoute.Environment).To(Equal(envName))
			Expect(beRoute.URL).NotTo(BeEmpty())
			Expect(beRoute.Ingress).To(Equal("environment"))

			Eventually(func() (int, error) {
				resp, fetchErr := http.Get(route.URL)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))
			Eventually(func() (int, error) {
				resp, fetchErr := http.Get(fmt.Sprintf("%s/proxy?url=%s/hello-world", route.URL, beRoute.URL))
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))
		})
	})

	It("svc logs should display logs", func() {
		var svcLogs []client.SvcLogsOutput
		var svcLogsErr error
		Eventually(func() ([]client.SvcLogsOutput, error) {
			svcLogs, svcLogsErr = cli.SvcLogs(&client.SvcLogsRequest{
				AppName: appName,
				Name:    feSvcName,
				EnvName: "test",
				Since:   "1h",
			})
			return svcLogs, svcLogsErr
		}, "300s", "10s").ShouldNot(BeEmpty())

		for _, logLine := range svcLogs {
			Expect(logLine.Message).NotTo(Equal(""))
			Expect(logLine.LogStreamName).NotTo(Equal(""))
			Expect(logLine.Timestamp).NotTo(Equal(0))
			Expect(logLine.IngestionTime).NotTo(Equal(0))
		}
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
				Name:    feSvcName,
			})
			pausedSvcStatus, pausedSvcStatusError = cli.SvcStatus(&client.SvcStatusRequest{
				Name:    feSvcName,
				AppName: appName,
				EnvName: envName,
			})
			_, svcResumeError = cli.SvcResume(&client.SvcResumeRequest{
				AppName: appName,
				EnvName: envName,
				Name:    feSvcName,
			})
			resumedSvcStatus, resumedSvcStatusError = cli.SvcStatus(&client.SvcStatusRequest{
				Name:    feSvcName,
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

	Context("force deploy the service", func() {
		var (
			svcDeployError error
			svcStatus      *client.SvcStatusOutput
			svcStatusError error
		)

		BeforeAll(func() {
			_, svcDeployError = cli.SvcDeploy(&client.SvcDeployInput{
				EnvName: envName,
				Name:    feSvcName,
				Force:   true,
			})
			svcStatus, svcStatusError = cli.SvcStatus(&client.SvcStatusRequest{
				Name:    feSvcName,
				AppName: appName,
				EnvName: envName,
			})
		})

		It("should not an return error", func() {
			Expect(svcDeployError).NotTo(HaveOccurred())
			Expect(svcStatusError).NotTo(HaveOccurred())
		})

		It("should successfully force deploy service", func() {
			Expect(svcStatus.Status).To(Equal("RUNNING"))
		})
	})

})
