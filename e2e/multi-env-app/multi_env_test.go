package multi_env_app_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

var (
	initErr error
)

var _ = Describe("Multiple Env App", func() {
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

	Context("when adding cross account environments", func() {
		var (
			testEnvInitErr error
			prodEnvInitErr error
		)
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: testEnvironmentProfile,
				Prod:    false,
			})

			_, prodEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "prod",
				Profile: prodEnvironmentProfile,
				Prod:    true,
			})

		})

		It("env init should succeed for test and prod envs", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
			Expect(prodEnvInitErr).NotTo(HaveOccurred())
		})

		It("env ls should list both envs", func() {
			envListOutput, err := cli.EnvList(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(envListOutput.Envs)).To(Equal(2))
			envs := map[string]client.EnvDescription{}
			for _, env := range envListOutput.Envs {
				envs[env.Name] = env
				Expect(env.ExecutionRole).NotTo(BeEmpty())
				Expect(env.ManagerRole).NotTo(BeEmpty())
			}

			Expect(envs["test"]).NotTo(BeNil())
			Expect(envs["test"].Prod).To(BeFalse())

			Expect(envs["prod"]).NotTo(BeNil())
			Expect(envs["prod"].Prod).To(BeTrue())

			// Make sure, for the sake of coverage, these are cross account,
			// cross region environments.
			Expect(envs["test"].Region).NotTo(Equal(envs["prod"].Region))
			Expect(envs["test"].Account).NotTo(Equal(envs["prod"].Account))
		})
	})

	Context("when adding a svc", func() {
		var (
			frontEndInitErr error
		)
		BeforeAll(func() {
			_, frontEndInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "front-end",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./front-end/Dockerfile",
				SvcPort:    "80",
			})
		})

		It("svc init should succeed", func() {
			Expect(frontEndInitErr).NotTo(HaveOccurred())
		})

		It("svc init should create a svc manifest", func() {
			Expect("./copilot/front-end/manifest.yml").Should(BeAnExistingFile())
		})

		It("svc ls should list the svc", func() {
			svcList, svcListError := cli.SvcList(appName)
			Expect(svcListError).NotTo(HaveOccurred())
			Expect(len(svcList.Services)).To(Equal(1))
			Expect(svcList.Services[0].Name).To(Equal("front-end"))
		})

		It("svc package should output a cloudformation template and params file", func() {
			Skip("not implemented yet")
		})
	})

	Context("when deploying a svc to test and prod envs", func() {
		var (
			testDeployErr    error
			prodEndDeployErr error
			svcName          string
		)
		BeforeAll(func() {
			svcName = "front-end"
			_, testDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     svcName,
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})

			_, prodEndDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     svcName,
				EnvName:  "prod",
				ImageTag: "gallopinggurdey",
			})
		})

		It("svc deploy should succeed to both environment", func() {
			Expect(testDeployErr).NotTo(HaveOccurred())
			Expect(prodEndDeployErr).NotTo(HaveOccurred())
		})

		It("svc show should include a valid URL and description for test and prod envs", func() {
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(2))
			// Group routes by environment
			envRoutes := map[string]client.SvcShowRoutes{}
			for _, route := range svc.Routes {
				envRoutes[route.Environment] = route
			}

			Expect(len(svc.ServiceDiscoveries)).To(Equal(1))
			Expect(svc.ServiceDiscoveries[0].Environment).To(Equal([]string{"test", "prod"}))
			Expect(svc.ServiceDiscoveries[0].Namespace).To(Equal(fmt.Sprintf("%s.%s.local:80", svc.SvcName, appName)))

			// Call each environment's endpoint and ensure it returns a 200
			for _, env := range []string{"test", "prod"} {
				route := envRoutes[env]
				Expect(route.Environment).To(Equal(env))
				Eventually(func() (int, error) {
					resp, fetchErr := http.Get(route.URL)
					return resp.StatusCode, fetchErr
				}, "30s", "1s").Should(Equal(200))
			}
		})

		It("svc logs should display logs", func() {
			for _, envName := range []string{"test", "prod"} {
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
			}
		})

		It("env show should display info for test and prod envs", func() {
			envs := map[string]client.EnvDescription{}
			for _, envName := range []string{"test", "prod"} {
				envShowOutput, envShowErr := cli.EnvShow(&client.EnvShowRequest{
					AppName: appName,
					EnvName: envName,
				})
				Expect(envShowErr).NotTo(HaveOccurred())
				Expect(envShowOutput.Environment.Name).To(Equal(envName))
				Expect(envShowOutput.Environment.App).To(Equal(appName))

				Expect(len(envShowOutput.Services)).To(Equal(1))
				Expect(envShowOutput.Services[0].Name).To(Equal(svcName))
				Expect(envShowOutput.Services[0].Type).To(Equal("Load Balanced Web Service"))

				Expect(len(envShowOutput.Tags)).To(Equal(2))
				Expect(envShowOutput.Tags["copilot-application"]).To(Equal(appName))
				Expect(envShowOutput.Tags["copilot-environment"]).To(Equal(envName))

				envs[envShowOutput.Environment.Name] = envShowOutput.Environment
			}
			Expect(envs["test"]).NotTo(BeNil())
			Expect(envs["test"].Prod).To(BeFalse())
			Expect(envs["prod"]).NotTo(BeNil())
			Expect(envs["prod"].Prod).To(BeTrue())
			Expect(envs["test"].Region).NotTo(Equal(envs["prod"].Region))
			Expect(envs["test"].Account).NotTo(Equal(envs["prod"].Account))
			Expect(envs["test"].ExecutionRole).NotTo(Equal(envs["prod"].ExecutionRole))
			Expect(envs["test"].ManagerRole).NotTo(Equal(envs["prod"].ExecutionRole))
		})
	})

	Context("when setting up a pipeline", func() {
		It("pipeline init should create a pipeline manifest", func() {
			Skip("not implemented yet")
		})

		It("pipeline update should create a pipeline", func() {
			Skip("not implemented yet")
		})
	})

	Context("when pushing a change to the pipeline", func() {
		It("the change should be propagated to test and prod environments", func() {
			Skip("not implemented yet")
		})
	})

})
