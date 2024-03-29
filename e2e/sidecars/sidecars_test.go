// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package sidecars_test

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const manifest = `# The manifest for the "hello" service.
# Read the full specification for the "Load Balanced Web Service" type at:
#  https://aws.github.io/copilot-cli/docs/manifest/lb-web-service/

# Your service name will be used in naming your resources like log groups, ECS services, etc.
name: hello
# The "architecture" of the service you're running.
type: Load Balanced Web Service

image:
  # Path to your service's Dockerfile.
  location: %s
  # Port exposed through your container to route traffic to it.
  port: 3000
  depends_on:
    nginx: start
env_file: ./magic.env

platform: linux/x86_64

http:
  # Requests to this path will be forwarded to your service. 
  # To match all requests you can use the "/" path. 
  path: 'api'
  # You can specify a custom health check path. The default is "/"
  healthcheck: '/api/health-check'
  targetContainer: 'nginx'

# Number of CPU units for the task.
cpu: 256
# Amount of memory in MiB used by the task.
memory: 512
# Number of tasks that should be running in your service.
count: 1
sidecars:
  nginx:
    port: 80
    image:
      build: nginx/Dockerfile
      context: ./nginx
    variables:
      NGINX_PORT: 80
    env_file: ./magic.env
logging:
  env_file: ./magic.env
  destination:
    Name: cloudwatch
    region: us-east-1
    log_group_name: /copilot/%s
    log_stream_prefix: copilot/
`

var mainImageURI string

var _ = Describe("sidecars flow", func() {
	Context("build and push main image to ECR repo", func() {
		var uri string
		var err error
		It("create new ECR repo for main container", func() {
			uri, err = aws.CreateECRRepo(mainRepoName)
			Expect(err).NotTo(HaveOccurred(), "create ECR repo for main container")
			mainImageURI = fmt.Sprintf("%s:mytag", uri)
		})
		It("push main container image", func() {
			var password string
			password, err = aws.ECRLoginPassword()
			Expect(err).NotTo(HaveOccurred(), "get ecr login password")
			err = docker.Login(uri, password)
			Expect(err).NotTo(HaveOccurred(), "docker login")
			err = docker.Build(mainImageURI, "./hello")
			Expect(err).NotTo(HaveOccurred(), "build main container image")
			err = docker.Push(mainImageURI)
			Expect(err).NotTo(HaveOccurred(), "push to ECR repo")
		})
	})

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

		It("app init creates an copilot directory", func() {
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

	Context("when adding a new environment", Ordered, func() {
		var (
			testEnvInitErr error
		)
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: envName,
				Profile: envName,
			})
		})

		It("env init should succeed", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environment", Ordered, func() {
		var envDeployErr error
		BeforeAll(func() {
			_, envDeployErr = cli.EnvDeploy(&client.EnvDeployRequest{
				AppName: appName,
				Name:    envName,
			})
		})

		It("should succeed", func() {
			Expect(envDeployErr).NotTo(HaveOccurred())
		})
	})

	Context("when creating a service", Ordered, func() {
		var (
			svcInitErr error
		)
		BeforeAll(func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:    svcName,
				SvcType: "Load Balanced Web Service",
				Image:   mainImageURI,
				SvcPort: "3000",
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

	Context("write local manifest and addon files", func() {
		var newManifest string
		It("overwrite existing manifest", func() {
			logGroupName := fmt.Sprintf("%s-test-%s", appName, svcName)
			newManifest = fmt.Sprintf(manifest, mainImageURI, logGroupName)
			err := os.WriteFile("./copilot/hello/manifest.yml", []byte(newManifest), 0644)
			Expect(err).NotTo(HaveOccurred(), "overwrite manifest")
		})
		It("add addons folder for Firelens permissions", func() {
			err := os.MkdirAll("./copilot/hello/addons", 0777)
			Expect(err).NotTo(HaveOccurred(), "create addons dir")

			fds, err := os.ReadDir("./hello/addons")
			Expect(err).NotTo(HaveOccurred(), "read addons dir")

			for _, fd := range fds {
				func() {
					destFile, err := os.Create(fmt.Sprintf("./copilot/hello/addons/%s", fd.Name()))
					Expect(err).NotTo(HaveOccurred(), "create destination file")
					defer destFile.Close()

					srcFile, err := os.Open(fmt.Sprintf("./hello/addons/%s", fd.Name()))
					Expect(err).NotTo(HaveOccurred(), "open source file")
					defer srcFile.Close()

					_, err = io.Copy(destFile, srcFile)
					Expect(err).NotTo(HaveOccurred(), "copy file")
				}()
			}
		})
	})

	Context("when deploying svc", Ordered, func() {
		var (
			appDeployErr error
		)
		BeforeAll(func() {
			_, appDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     svcName,
				EnvName:  envName,
				ImageTag: "gallopinggurdey",
			})
		})

		It("svc deploy should succeed", func() {
			Expect(appDeployErr).NotTo(HaveOccurred())
		})

		It("svc show should include a valid URL and description for test env", func() {
			svc, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    svcName,
			})
			Expect(svcShowErr).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			// Call each environment's endpoint and ensure it returns a 200
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal(envName))
			uri := route.URL + "/health-check"

			// Service should be ready.
			resp := &http.Response{}
			var fetchErr error
			Eventually(func() (int, error) {
				resp, fetchErr = http.Get(uri)
				return resp.StatusCode, fetchErr
			}, "30s", "1s").Should(Equal(200))

			// Read the response - our deployed apps should return a body with their
			// name as the value.
			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(bodyBytes)).To(Equal("Ready"))
		})

		It("should have env file in sidecar definitions", func() {
			taskDefinitionName := fmt.Sprintf("%s-%s-%s", appName, envName, svcName)
			envFiles, err := aws.GetEnvFilesFromTaskDefinition(taskDefinitionName)
			Expect(err).NotTo(HaveOccurred())
			Expect(envFiles).To(HaveKey(svcName))
			Expect(envFiles).To(HaveKey("nginx"))
			Expect(envFiles).To(HaveKey("firelens_log_router"))
		})

		It("svc logs should display logs", func() {
			var firelensCreated bool
			var svcLogs []client.SvcLogsOutput
			var svcLogsErr error
			for i := 0; i < 10; i++ {
				svcLogs, svcLogsErr = cli.SvcLogs(&client.SvcLogsRequest{
					AppName: appName,
					Name:    svcName,
					EnvName: envName,
					Since:   "1h",
				})
				if svcLogsErr != nil {
					exponentialBackoffWithJitter(i)
					continue
				}
				for _, logLine := range svcLogs {
					if strings.Contains(logLine.LogStreamName, fmt.Sprintf("copilot/%s-firelens-", svcName)) {
						firelensCreated = true
					}
				}
				if firelensCreated {
					break
				}
				exponentialBackoffWithJitter(i)
			}
			Expect(svcLogsErr).NotTo(HaveOccurred())
			Expect(firelensCreated).To(Equal(true))
			// Randomly check if a log line is valid.
			logLine := rand.Intn(len(svcLogs))
			Expect(svcLogs[logLine].Message).NotTo(Equal(""))
			Expect(svcLogs[logLine].LogStreamName).NotTo(Equal(""))
			Expect(svcLogs[logLine].Timestamp).NotTo(Equal(0))
			Expect(svcLogs[logLine].IngestionTime).NotTo(Equal(0))
		})
	})
})
