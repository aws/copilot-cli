// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons_test

import (
	"fmt"
	"net/http"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("addons flow", func() {
	Context("when creating a new app", Ordered, func() {
		var (
			initErr error
		)
		BeforeAll(func() {
			_, initErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
		})

		It("app init succeeds", func() {
			Expect(initErr).NotTo(HaveOccurred())
		})

		It("app init creates an copilot directory and workspace file", func() {
			Expect("./copilot").Should(BeADirectory())
			Expect("./copilot/.workspace").Should(BeAnExistingFile())
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

	Context("when adding a new environment", Ordered, func() {
		var (
			testEnvInitErr error
		)
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: "test",
			})
		})

		It("should succeed", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environment", Ordered, func() {
		var envDeployErr error
		BeforeAll(func() {
			_, envDeployErr = cli.EnvDeploy(&client.EnvDeployRequest{
				AppName: appName,
				Name:    "test",
			})
		})

		It("should succeed", func() {
			Expect(envDeployErr).NotTo(HaveOccurred())
		})
	})

	Context("when adding a svc", Ordered, func() {
		var (
			svcInitErr error
		)
		BeforeAll(func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       svcName,
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./hello/Dockerfile",
				SvcPort:    "80",
			})
		})

		It("svc init should succeed", func() {
			Expect(svcInitErr).NotTo(HaveOccurred())
		})

		It("svc init should create svc manifests", func() {
			Expect("./copilot/hello/manifest.yml").Should(BeAnExistingFile())

		})

		It("svc ls should list the service", func() {
			svcList, svcListError := cli.SvcList(appName)
			Expect(svcListError).NotTo(HaveOccurred())
			Expect(len(svcList.Services)).To(Equal(1))

			svcsByName := map[string]client.WkldDescription{}
			for _, svc := range svcList.Services {
				svcsByName[svc.Name] = svc
			}

			for _, svc := range []string{svcName} {
				Expect(svcsByName[svc].AppName).To(Equal(appName))
				Expect(svcsByName[svc].Name).To(Equal(svc))
			}
		})
	})

	Context("when adding an RDS storage", Ordered, func() {
		var testStorageInitErr error
		BeforeAll(func() {
			_, testStorageInitErr = cli.StorageInit(&client.StorageInitRequest{
				StorageName:   rdsStorageName,
				StorageType:   rdsStorageType,
				WorkloadName:  svcName,
				Lifecycle:     "workload",
				RDSEngine:     rdsEngine,
				InitialDBName: rdsInitialDB,
			})
		})

		It("storage init should succeed", func() {
			Expect(testStorageInitErr).NotTo(HaveOccurred())
		})

		It("storage init should create an addon template", func() {
			addonFilePath := fmt.Sprintf("./copilot/%s/addons/%s.yml", svcName, rdsStorageName)
			Expect(addonFilePath).Should(BeAnExistingFile())
		})
	})

	Context("when adding a S3 storage", Ordered, func() {
		var testStorageInitErr error
		BeforeAll(func() {
			_, testStorageInitErr = cli.StorageInit(&client.StorageInitRequest{
				StorageName:  s3StorageName,
				StorageType:  s3StorageType,
				WorkloadName: svcName,
				Lifecycle:    "workload",
			})
		})

		It("storage init should succeed", func() {
			Expect(testStorageInitErr).NotTo(HaveOccurred())
		})

		It("storage init should create an addon template", func() {
			addonFilePath := fmt.Sprintf("./copilot/%s/addons/%s.yml", svcName, s3StorageName)
			Expect(addonFilePath).Should(BeAnExistingFile())
		})
	})

	Context("when deploying svc", Ordered, func() {
		var (
			svcDeployErr error
			svcInitErr   error
			initErr      error
		)

		BeforeAll(func() {
			_, svcDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     svcName,
				EnvName:  "test",
				ImageTag: "gallopinggurdey",
			})
		})

		It("svc deploy should succeed", func() {
			Expect(svcDeployErr).NotTo(HaveOccurred())
		})

		It("should be able to make a GET request", func() {
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			// Make a GET request to the API.
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal("test"))
			Eventually(func() (int, error) {
				resp, fetchErr := http.Get(route.URL)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))
		})

		It("initial database should have been created for the Aurora stroage", func() {
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			// Make a GET request to the API to make sure initial database exists.
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal("test"))

			endpoint := fmt.Sprintf("%s/%s", "databases", rdsInitialDB)
			Eventually(func() (int, error) {
				url := fmt.Sprintf("%s/%s", route.URL, endpoint)
				resp, fetchErr := http.Get(url)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))
		})

		It("should be able to peek into s3 bucket", func() {
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			// Make a GET request to the API to make sure we can access the s3 bucket.
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal("test"))

			Eventually(func() (int, error) {
				url := fmt.Sprintf("%s/%s", route.URL, "peeks3")
				resp, fetchErr := http.Get(url)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))
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

		It("svc delete should not delete local files", func() {
			_, err := cli.SvcDelete(svcName)
			Expect(err).NotTo(HaveOccurred())
			Expect("./copilot/hello/addons").Should(BeADirectory())
			Expect("./copilot/hello/manifest.yml").Should(BeAnExistingFile())
			Expect("./copilot/.workspace").Should(BeAnExistingFile())

			// Need to recreate the service for AfterSuite testing.
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       svcName,
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./hello/Dockerfile",
				SvcPort:    "80",
			})
			Expect(svcInitErr).NotTo(HaveOccurred())
		})

		It("app delete does remove .workspace but keep local files", func() {
			_, err := cli.AppDelete()
			Expect(err).NotTo(HaveOccurred())
			Expect("./copilot").Should(BeADirectory())
			Expect("./copilot/hello/addons").Should(BeADirectory())
			Expect("./copilot/hello/manifest.yml").Should(BeAnExistingFile())
			Expect("./copilot/environments/test/manifest.yml").Should(BeAnExistingFile())
			Expect("./copilot/.workspace").ShouldNot(BeAnExistingFile())

			// Need to recreate the app for AfterSuite testing.
			_, initErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
			Expect(initErr).NotTo(HaveOccurred())
		})
	})
})
