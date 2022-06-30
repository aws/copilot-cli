// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package isolated_test

import (
	"errors"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	osExec "os/exec"

	"os"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

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
			if vpcImport.ID == "" || vpcImport.PrivateSubnetIDs == "" {
				err = errors.New("resources are not configured properly")
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
			Expect(svc.Configs[0].CPU).To(Equal("256"))
			Expect(svc.Configs[0].Memory).To(Equal("512"))
			Expect(svc.Configs[0].Port).To(Equal("80"))
		})

		Context("when running `svc logs`", func() {
			It("logs should be displayed", func() {
				var svcLogs []client.SvcLogsOutput
				var svcLogsErr error
				Eventually(func() ([]client.SvcLogsOutput, error) {
					svcLogs, svcLogsErr = cli.SvcLogs(&client.SvcLogsRequest{
						AppName: appName,
						Name:    svcName,
						EnvName: envName,
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

		Context("when running `env show --resources`", func() {
			var envShowOutput *client.EnvShowOutput
			var envShowErr error
			It("show show internal ALB", func() {
				envShowOutput, envShowErr = cli.EnvShow(&client.EnvShowRequest{
					AppName: appName,
					EnvName: envName,
				})
			})
			It("should not return an error", func() {
				Expect(envShowErr).NotTo(HaveOccurred())
			})
			It("should now have an internal ALB", func() {
				Expect(envShowOutput.Resources).To(ContainElement(HaveKeyWithValue("type", "AWS::ElasticLoadBalancingV2::LoadBalancer")))
			})
		})

		Context("when `curl`ing the LB DNS", func() {
			It("is not reachable", func() {
				cmd := osExec.Command("curl", fmt.Sprintf("http://%s.%s.%s.internal", svcName, envName, appName))
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				Expect(cmd.Stderr).NotTo(BeNil())
			})
		})

		Context("when `curl`ing the LB DNS from within the container", func() {
			It("session manager should be installed", func() {
				// Use custom SSM plugin as the public version is not compatible to Alpine Linux.
				err := client.BashExec("chmod +x ./session-manager-plugin")
				Expect(err).NotTo(HaveOccurred())
			})
			It("is reachable", func() {
				_, svcExecErr := cli.SvcExec(&client.SvcExecRequest{
					Name:    svcName,
					AppName: appName,
					Command: fmt.Sprintf(`/bin/sh -c "curl 'http://%s.%s.%s.internal'"`, svcName, envName, appName),
					EnvName: envName,
				})
				Expect(svcExecErr).NotTo(HaveOccurred())
			})
		})
	})
})
