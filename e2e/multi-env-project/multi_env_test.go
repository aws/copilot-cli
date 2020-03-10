package multi_env_app_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/amazon-ecs-cli-v2/e2e/internal/client"
)

var (
	initErr error
)

var _ = Describe("Multiple Env Project", func() {
	Context("when creating a new project", func() {
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

	Context("when adding cross account environments", func() {
		var (
			testEnvInitErr error
			prodEnvInitErr error
		)
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				ProjectName: projectName,
				EnvName:     "test",
				Profile:     testEnvironmentProfile,
				Prod:        false,
			})

			_, prodEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				ProjectName: projectName,
				EnvName:     "prod",
				Profile:     prodEnvironmentProfile,
				Prod:        true,
			})

		})

		It("env init should succeed for test and prod envs", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
			Expect(prodEnvInitErr).NotTo(HaveOccurred())
		})

		It("env ls should list both envs", func() {
			envListOutput, err := cli.EnvList(projectName)
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

	Context("when adding an app", func() {
		var (
			frontEndInitErr error
		)
		BeforeAll(func() {
			_, frontEndInitErr = cli.AppInit(&client.AppInitRequest{
				AppName:    "front-end",
				AppType:    "Load Balanced Web App",
				Dockerfile: "./front-end/Dockerfile",
				AppPort:    "80",
			})
		})

		It("app init should succeed", func() {
			Expect(frontEndInitErr).NotTo(HaveOccurred())
		})

		It("app init should create an app manifest", func() {
			Expect("./ecs-project/front-end/manifest.yml").Should(BeAnExistingFile())
		})

		It("app ls should list the app", func() {
			appList, appListError := cli.AppList(projectName)
			Expect(appListError).NotTo(HaveOccurred())
			Expect(len(appList.Apps)).To(Equal(1))
			Expect(appList.Apps[0].AppName).To(Equal("front-end"))
		})

		It("app package should output a cloudformation template and params file", func() {
			Skip("not implemented yet")
		})
	})

	Context("when deploying an app to test and prod envs", func() {
		var (
			testDeployErr    error
			prodEndDeployErr error
			appName          string
		)
		BeforeAll(func() {
			appName = "front-end"
			_, testDeployErr = cli.AppDeploy(&client.AppDeployInput{
				AppName:  appName,
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})

			_, prodEndDeployErr = cli.AppDeploy(&client.AppDeployInput{
				AppName:  appName,
				EnvName:  "prod",
				ImageTag: "gallopinggurdey",
			})
		})

		It("app deploy should succeed to both environment", func() {
			Expect(testDeployErr).NotTo(HaveOccurred())
			Expect(prodEndDeployErr).NotTo(HaveOccurred())
		})

		It("app show should include a valid URL and description for test and prod envs", func() {
			app, appShowErr := cli.AppShow(&client.AppShowRequest{
				ProjectName: projectName,
				AppName:     appName,
			})
			Expect(appShowErr).NotTo(HaveOccurred())
			Expect(len(app.Routes)).To(Equal(2))
			// Group routes by environment
			envRoutes := map[string]client.AppShowRoutes{}
			for _, route := range app.Routes {
				envRoutes[route.Environment] = route
			}

			// Call each environment's endpoint and ensure it returns a 200
			for _, env := range []string{"test", "prod"} {
				route := envRoutes[env]
				Expect(route.Environment).To(Equal(env))
				Expect(route.URL).To(Equal(appName))
				Eventually(func() (int, error) {
					resp, fetchErr := http.Get(fmt.Sprintf("http://%s/", route.URL))
					return resp.StatusCode, fetchErr
				}, "10s", "1s").Should(Equal(200))
			}
		})

		It("app logs should display logs", func() {
			for _, envName := range []string{"test", "prod"} {
				var appLogs []client.AppLogsOutput
				var appLogsErr error
				Eventually(func() ([]client.AppLogsOutput, error) {
					appLogs, appLogsErr = cli.AppLogs(&client.AppLogsRequest{
						ProjectName: projectName,
						AppName:     appName,
						EnvName:     envName,
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
			}
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
