// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package isolated_test

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

type countAssertionTracker struct {
	expected int
	actual   int
}

var _ = Describe("Isolated", func() {
	Context("when creating a new app", func() {
		var appInitErr error
		BeforeAll(func() {
			_, appInitErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
				Tags: map[string]string{
					"e2e-test": "isolated",
				},
			})
		})

		It("app init succeeds", func() {
			Expect(appInitErr).NotTo(HaveOccurred())
		})

		It("app init creates a copilot directory", func() {
			Expect("./copilot").Should(BeADirectory())
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

	Context("when deploying resources to be imported", func() {
		BeforeAll(func() {
			err := aws.CreateStack(vpcStackName, vpcStackTemplatePath)
			Expect(err).NotTo(HaveOccurred(), "create vpc cloudformation stack")
			err = aws.WaitStackCreateComplete(vpcStackName)
			Expect(err).NotTo(HaveOccurred(), "vpc stack create complete")
		})
		It("parse vpc stack output", func() {
			outputs, err := aws.VPCStackOutput(vpcStackName)
			Expect(err).NotTo(HaveOccurred(), "get VPC stack output")
			for _, output := range outputs {
				switch output.OutputKey {
				case "PrivateSubnets":
					vpcImport.PrivateSubnetIDs = output.OutputValue
				case "VpcId":
					vpcImport.ID = output.OutputValue
				}
			}
			if !vpcImport.IsSet() {
				err = errors.New("vpc resources are not configured properly")
			}
			Expect(err).NotTo(HaveOccurred(), "invalid vpc stack output")
		})
	})

	Context("when adding environment with imported vpc resources", func() {
		var testEnvInitErr error
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName:       appName,
				EnvName:       envName,
				Profile:       "default",
				Prod:          false,
				VPCImport:     vpcImport,
				CustomizedEnv: true,
			})
		})

		It("env init should succeed for 'private' env", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
		})

		It("env ls should list private env", func() {
			envListOutput, err := cli.EnvList(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(envListOutput.Envs)).To(Equal(1))
			env := envListOutput.Envs[0]
			Expect(env.Name).To(Equal(envName))
			Expect(env.Prod).To(BeFalse())
			Expect(env.ExecutionRole).NotTo(BeEmpty())
			Expect(env.ManagerRole).NotTo(BeEmpty())
		})
	})

	Context("when creating and deploying a backend service in private subnets", func() {
		var initErr error

		BeforeAll(func() {
			_, initErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       svcName,
				SvcType:    "Backend Service",
				Dockerfile: "./backend/Dockerfile",
				SvcPort:    "80",
			})
		})

		It("should not return an error", func() {
			Expect(initErr).NotTo(HaveOccurred())
		})
		It("svc init should create a svc manifest", func() {
			Expect("./copilot/backend/manifest.yml").Should(BeAnExistingFile())
		})
		It("should write 'http' and private placement to the manifest", func() {
			f, err := os.OpenFile("./copilot/backend/manifest.yml", os.O_WRONLY|os.O_APPEND, 0644)
			Expect(err).NotTo(HaveOccurred(), "should be able to open the file to append content")
			_, err = f.WriteString(`
http:
  path: '/'
network:
  vpc:
    placement: 'private'
`)
			Expect(err).NotTo(HaveOccurred(), "should be able to write 'private' placement to manifest file")
			err = f.Close()
			Expect(err).NotTo(HaveOccurred(), "should have been able to close the manifest file")
		})
		It("svc ls should list the svc", func() {
			svcList, svcListError := cli.SvcList(appName)
			Expect(svcListError).NotTo(HaveOccurred())
			Expect(len(svcList.Services)).To(Equal(1))
			Expect(svcList.Services[0].Name).To(Equal("backend"))
		})
	})

	Context("when deploying a svc to 'private' env", func() {
		var privateEnvDeployErr error
		BeforeAll(func() {
			_, privateEnvDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:    svcName,
				EnvName: envName,
			})
		})

		It("svc deploy should succeed", func() {
			Expect(privateEnvDeployErr).NotTo(HaveOccurred())
		})
	})

	Context("when running svc show to retrieve the service configuration, resources, and endpoint, then querying the service", func() {
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

		It("should return correct, working route", func() {
			Expect(svc.Routes[0].Environment).To(Equal(envName))
			//Expect(svc.Routes[0].URL).NotTo(BeEmpty())
			Expect(svc.Routes[0].URL).To(Equal("TKTK"))
			Eventually(func() (int, error) {
				resp, fetchErr := http.Get(svc.Routes[0].URL)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))
		})

		It("should return correct, working service discovery namespace", func() {
			Expect(svc.ServiceDiscoveries[0].Environment).To(Equal(envName))
			Expect(svc.ServiceDiscoveries[0].Namespace).To(Equal(fmt.Sprintf("%s.%s.local", envName, appName)))
			Eventually(func() (int, error) {
				resp, fetchErr := http.Get(svc.ServiceDiscoveries[0].Namespace)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))
		})

		It("should return correct environment variables", func() {
			fmt.Printf("\n\nenvironment variables: %+v\n\n", svc.Variables)
			expectedVars := map[string]string{
				"COPILOT_APPLICATION_NAME":           appName,
				"COPILOT_ENVIRONMENT_NAME":           envName,
				"COPILOT_SERVICE_NAME":               svcName,
				"COPILOT_SERVICE_DISCOVERY_ENDPOINT": fmt.Sprintf("%s.%s.local", envName, appName),
			}
			for _, variable := range svc.Variables {
				Expect(variable.Value).To(Equal(expectedVars[variable.Name]))
			}
		})

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
}