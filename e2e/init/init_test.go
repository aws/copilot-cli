package init_test

import (
	"fmt"
	"net/http"

	"github.com/aws/amazon-ecs-cli-v2/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("init flow", func() {

	var (
		appName string
		initErr error
	)

	BeforeAll(func() {
		appName = "front-end"
		_, initErr = cli.Init(&client.InitRequest{
			ProjectName: projectName,
			AppName:     appName,
			ImageTag:    "gallopinggurdey",
			Dockerfile:  "./front-end/Dockerfile",
			AppType:     "Load Balanced Web App",
			Deploy:      true,
			AppPort:     "80",
		})
	})

	Context("creating a brand new project, app and deploying to a test environment", func() {
		It("init does not return an error", func() {
			Expect(initErr).NotTo(HaveOccurred())
		})
	})

	Context("app ls", func() {
		var (
			appList      *client.AppListOutput
			appListError error
		)

		BeforeAll(func() {
			appList, appListError = cli.AppList(projectName)
		})

		It("should not return an error", func() {
			Expect(appListError).NotTo(HaveOccurred())
		})

		It("should return one app", func() {
			Expect(len(appList.Apps)).To(Equal(1))
			Expect(appList.Apps[0].AppName).To(Equal(appName))
			Expect(appList.Apps[0].Project).To(Equal(projectName))
		})
	})

	Context("app show", func() {
		var (
			app        *client.AppShowOutput
			appShowErr error
		)

		BeforeAll(func() {
			app, appShowErr = cli.AppShow(&client.AppShowRequest{
				ProjectName: projectName,
				AppName:     appName,
			})

		})

		It("should not return an error", func() {
			Expect(appShowErr).NotTo(HaveOccurred())
		})

		It("should return the correct configuration", func() {
			Expect(app.AppName).To(Equal(appName))
			Expect(app.Project).To(Equal(projectName))
		})

		It("should return a valid route", func() {
			Expect(len(app.Routes)).To(Equal(1))
			Expect(app.Routes[0].Environment).To(Equal("test"))
			Expect(app.Routes[0].Path).To(Equal(appName))
			resp, fetchErr := http.Get(fmt.Sprintf("http://%s/", app.Routes[0].URL))
			Expect(fetchErr).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))
		})

		It("should return the correct environment variables", func() {
			Expect(len(app.Variables)).To(Equal(4))
			expectedVars := map[string]string{
				"ECS_CLI_APP_NAME":         appName,
				"ECS_CLI_ENVIRONMENT_NAME": "test",
				"ECS_CLI_LB_DNS":           app.Routes[0].URL,
				"ECS_CLI_PROJECT_NAME":     projectName,
			}
			for _, variable := range app.Variables {
				Expect(variable.Value).To(Equal(expectedVars[variable.Name]))
			}
		})
	})

	Context("app logs", func() {

		It("should return valid log lines", func() {
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
		})
	})
})
