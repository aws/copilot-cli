// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package isolated_test

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Isolated", func() {
	Context("when creating a new app", Ordered, func() {
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
		It("vpc stack exists", func() {
			err := aws.WaitStackCreateComplete(vpcStackName)
			Expect(err).NotTo(HaveOccurred())
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

	Context("when adding environment with imported vpc resources", Ordered, func() {
		var testEnvInitErr error
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName:       appName,
				EnvName:       envName,
				Profile:       envName,
				VPCImport:     vpcImport,
				CustomizedEnv: true,
			})
		})
		It("env init should succeed for 'test' env", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environment", Ordered, func() {
		var testEnvDeployErr error
		BeforeAll(func() {
			_, testEnvDeployErr = cli.EnvDeploy(&client.EnvDeployRequest{
				AppName: appName,
				Name:    envName,
			})
		})
		It("should succeed", func() {
			Expect(testEnvDeployErr).NotTo(HaveOccurred())
		})
		It("env ls should list test env", func() {
			envListOutput, err := cli.EnvList(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(envListOutput.Envs)).To(Equal(1))
			env := envListOutput.Envs[0]
			Expect(env.Name).To(Equal(envName))
			Expect(env.ExecutionRole).NotTo(BeEmpty())
			Expect(env.ManagerRole).NotTo(BeEmpty())
		})
	})

	Context("when creating a backend service in private subnets", Ordered, func() {
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
		It("svc ls should list the svc", func() {
			svcList, svcListError := cli.SvcList(appName)
			Expect(svcListError).NotTo(HaveOccurred())
			Expect(len(svcList.Services)).To(Equal(1))
			Expect(svcList.Services[0].Name).To(Equal("backend"))
		})
	})

	Context("when deploying a svc to 'test' env", Ordered, func() {
		var testEnvDeployErr error
		BeforeAll(func() {
			_, testEnvDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:    svcName,
				EnvName: envName,
			})
		})

		It("svc deploy should succeed", func() {
			Expect(testEnvDeployErr).NotTo(HaveOccurred())
		})
	})

	Context("when running svc show to retrieve the service configuration, resources, and endpoint, then querying the service", Ordered, func() {
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
			Expect(svc.Routes[0].URL).To(ContainSubstring(fmt.Sprintf("http://%s.%s.%s.internal", svcName, envName, appName)))
		})
	})

	Context("when running `svc status`", func() {
		It("it should include the service, tasks, and alarm status", func() {
			svc, svcStatusErr := cli.SvcStatus(&client.SvcStatusRequest{
				AppName: appName,
				Name:    svcName,
				EnvName: envName,
			})
			Expect(svcStatusErr).NotTo(HaveOccurred())
			// Service should be active.
			Expect(svc.Service.Status).To(Equal("ACTIVE"))
			// Desired count should be minimum auto scaling number.
			Expect(svc.Service.DesiredCount).To(Equal(int64(1)))
			// Should have correct number of running tasks.
			Expect(len(svc.Tasks)).To(Equal(1))
		})
	})

	Context("when running `env show --resources`", func() {
		var envShowOutput *client.EnvShowOutput
		var envShowErr error
		It("should show internal ALB", func() {
			envShowOutput, envShowErr = cli.EnvShow(&client.EnvShowRequest{
				AppName: appName,
				EnvName: envName,
			})
			Expect(envShowErr).NotTo(HaveOccurred())
			Expect(envShowOutput.Resources).To(ContainElement(HaveKeyWithValue("type", "AWS::ElasticLoadBalancingV2::LoadBalancer")))
		})
	})

	Context("when trying to reach the LB DNS", func() {
		It("it is not reachable", func() {
			var resp *http.Response
			var fetchErr error
			Eventually(func() (*http.Response, error) {
				resp, fetchErr = http.Get(fmt.Sprintf("http://%s.%s.%s.internal", svcName, envName, appName))
				return resp, fetchErr
			}, "60s", "1s")
			Expect(resp).To(BeNil())
		})
	})

	Context("when `curl`ing the LB DNS from within the container", func() {
		It("session manager should be installed", func() {
			// Use custom SSM plugin as the public version is not compatible to Alpine Linux.
			err := client.BashExec("chmod +x ./session-manager-plugin")
			Expect(err).NotTo(HaveOccurred())
			err = client.BashExec("mv ./session-manager-plugin /bin/session-manager-plugin")
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
