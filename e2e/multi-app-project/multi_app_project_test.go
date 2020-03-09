package multi_app_project_test

import (
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/amazon-ecs-cli-v2/e2e/internal/client"
)

var (
	initErr error
)

var _ = Describe("Multiple App Project", func() {
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
			frontEndInitErr error
			backEndInitErr  error
		)
		BeforeAll(func() {
			_, frontEndInitErr = cli.AppInit(&client.AppInitRequest{
				AppName:    "front-end",
				AppType:    "Load Balanced Web App",
				Dockerfile: "./front-end/Dockerfile",
				AppPort:    "80",
			})

			_, backEndInitErr = cli.AppInit(&client.AppInitRequest{
				AppName:    "back-end",
				AppType:    "Load Balanced Web App",
				Dockerfile: "./back-end/Dockerfile",
				AppPort:    "80",
			})
		})

		It("app init should succeed", func() {
			Expect(frontEndInitErr).NotTo(HaveOccurred())
			Expect(backEndInitErr).NotTo(HaveOccurred())
		})

		It("app init should create app manifests", func() {
			Expect("./ecs-project/front-end/manifest.yml").Should(BeAnExistingFile())
			Expect("./ecs-project/back-end/manifest.yml").Should(BeAnExistingFile())

		})

		It("app ls should list the apps", func() {
			appList, appListError := cli.AppList(projectName)
			Expect(appListError).NotTo(HaveOccurred())
			Expect(len(appList.Apps)).To(Equal(2))

			appsByName := map[string]client.AppDescription{}
			for _, app := range appList.Apps {
				appsByName[app.AppName] = app
			}

			for _, app := range []string{"front-end", "back-end"} {
				Expect(appsByName[app].AppName).To(Equal(app))
				Expect(appsByName[app].Project).To(Equal(projectName))
			}
		})

		It("app package should output a cloudformation template and params file", func() {
			Skip("not implemented yet")
		})
	})

	Context("when deploying apps", func() {
		var (
			frontEndDeployErr error
			backEndDeployErr  error
		)
		BeforeAll(func() {
			_, frontEndDeployErr = cli.AppDeploy(&client.AppDeployInput{
				AppName:  "front-end",
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})

			_, backEndDeployErr = cli.AppDeploy(&client.AppDeployInput{
				AppName:  "back-end",
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})
		})

		It("app deploy should succeed to both environment", func() {
			Expect(frontEndDeployErr).NotTo(HaveOccurred())
			Expect(backEndDeployErr).NotTo(HaveOccurred())
		})

		It("app show should include a valid URL and description for test and prod envs", func() {
			for _, appName := range []string{"front-end", "back-end"} {
				app, appShowErr := cli.AppShow(&client.AppShowRequest{
					ProjectName: projectName,
					AppName:     appName,
				})
				Expect(appShowErr).NotTo(HaveOccurred())
				Expect(len(app.Routes)).To(Equal(1))

				// Call each environment's endpoint and ensure it returns a 200
				route := app.Routes[0]
				Expect(route.Environment).To(Equal("test"))
				Expect(route.URL).To(Equal(appName))
				resp, fetchErr := http.Get(fmt.Sprintf("http://%s/", route.URL))
				Expect(fetchErr).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				// Read the response - our deployed apps should return a body with their
				// name as the value.
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(bodyBytes)).To(Equal(appName))
			}
		})

		It("app logs should display logs", func() {
			for _, appName := range []string{"front-end", "back-end"} {
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
			}
		})
	})
})
