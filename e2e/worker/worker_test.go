// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker Service E2E Test", func() {
	Context("create an application", func() {
		It("app init succeeds", func() {
			_, err := cli.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("app init creates a copilot directory", func() {
			Expect("./copilot").Should(BeADirectory())
		})

		It("app show includes app name", func() {
			appShowOutput, err := cli.AppShow(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(appShowOutput.Name).To(Equal(appName))
			Expect(appShowOutput.URI).To(BeEmpty())
		})
	})

	Context("add an environment", func() {
		It("env init should succeed", func() {
			_, err := cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: envName,
				Profile: envName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environment", func() {
		It("env deploy should succeed", func() {
			_, err := cli.EnvDeploy(&client.EnvDeployRequest{
				AppName: appName,
				Name:    envName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("create a load balanced web service", func() {
		It("svc init should succeed", func() {
			_, err := cli.SvcInit(&client.SvcInitRequest{
				Name:       lbwsServiceName,
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./frontend/Dockerfile",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds a topic to push to the service", func() {
			Expect("./copilot/frontend/manifest.yml").Should(BeAnExistingFile())
			f, err := os.OpenFile("./copilot/frontend/manifest.yml", os.O_APPEND|os.O_WRONLY, 0644)
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()
			// Append publish section to manifest.
			_, err = f.WriteString(`
publish:
  topics:
  - name: events
`)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deploys the load balanced web service", func() {
			_, err := cli.SvcDeploy(&client.SvcDeployInput{
				Name:     lbwsServiceName,
				EnvName:  envName,
				ImageTag: "gallopinggurdey",
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("deploys the worker service", func() {
		It("svc init should succeed", func() {
			_, err := cli.SvcInit(&client.SvcInitRequest{
				Name:               workerServiceName,
				SvcType:            "Worker Service",
				Dockerfile:         "./worker/Dockerfile",
				TopicSubscriptions: []string{fmt.Sprintf("%s:%s", lbwsServiceName, "events")},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds a topic to push to the service", func() {
			Expect("./copilot/worker/manifest.yml").Should(BeAnExistingFile())
			f, err := os.OpenFile("./copilot/worker/manifest.yml", os.O_APPEND|os.O_WRONLY, 0644)
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()
			// Append publish section to manifest.
			_, err = f.WriteString(`
publish:
  topics:
  - name: processed-msg-count
`)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deploys the worker service", func() {
			_, err := cli.SvcDeploy(&client.SvcDeployInput{
				Name:     workerServiceName,
				EnvName:  envName,
				ImageTag: "gallopinggurdey",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should have the SQS queue and service discovery injected as env vars", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    workerServiceName,
			})
			Expect(err).NotTo(HaveOccurred())
			var hasQueueURI bool
			var svcDiscovery bool
			var hasTopic bool
			for _, envVar := range svc.Variables {
				switch envVar.Name {
				case "COPILOT_QUEUE_URI":
					hasQueueURI = true
				case "COPILOT_SERVICE_DISCOVERY_ENDPOINT":
					svcDiscovery = true
				case "COPILOT_SNS_TOPIC_ARNS":
					hasTopic = true
				}
			}
			if !hasQueueURI {
				Expect(errors.New("worker service is missing env var 'COPILOT_QUEUE_URI'")).NotTo(HaveOccurred())
			}
			if !svcDiscovery {
				Expect(errors.New("worker service is missing env var 'COPILOT_SERVICE_DISCOVERY_ENDPOINT'")).NotTo(HaveOccurred())
			}
			if !hasTopic {
				Expect(errors.New("worker service is missing env var 'COPILOT_SNS_TOPIC_ARNS'")).NotTo(HaveOccurred())
			}
		})
	})

	Context("deploys the counter service", func() {
		It("svc init should succeed", func() {
			_, err := cli.SvcInit(&client.SvcInitRequest{
				Name:               counterServiceName,
				SvcType:            "Worker Service",
				Dockerfile:         "./counter/Dockerfile",
				TopicSubscriptions: []string{fmt.Sprintf("%s:%s", workerServiceName, "processed-msg-count")},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("deploys the counter service", func() {
			_, err := cli.SvcDeploy(&client.SvcDeployInput{
				Name:     counterServiceName,
				EnvName:  envName,
				ImageTag: "gallopinggurdey",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should have the SQS queue and service discovery injected as env vars", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    workerServiceName,
			})
			Expect(err).NotTo(HaveOccurred())
			var hasQueueURI bool
			var svcDiscovery bool
			for _, envVar := range svc.Variables {
				switch envVar.Name {
				case "COPILOT_QUEUE_URI":
					hasQueueURI = true
				case "COPILOT_SERVICE_DISCOVERY_ENDPOINT":
					svcDiscovery = true
				}
			}
			if !hasQueueURI {
				Expect(errors.New("worker service is missing env var 'COPILOT_QUEUE_URI'")).NotTo(HaveOccurred())
			}
			if !svcDiscovery {
				Expect(errors.New("worker service is missing env var 'COPILOT_SERVICE_DISCOVERY_ENDPOINT'")).NotTo(HaveOccurred())
			}
		})
	})

	Context("should have consumed messages", func() {
		It("frontend service should have received acknowledgement", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    lbwsServiceName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(svc.Routes)).To(Equal(1))
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal(envName))
			Eventually(func() error {
				resp, err := http.Get(fmt.Sprintf("%s/status", route.URL))
				if err != nil {
					return err
				}
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("response status is %d and not %d", resp.StatusCode, http.StatusOK)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				fmt.Printf("response: %s\n", string(body))
				// The LBWS sets its state to "consumed" when the worker service processes a message
				if string(body) != "consumed" {
					return fmt.Errorf("the message content is '%s', but expected '%s'", string(body), "consumed")
				}
				return nil
			}, "60s", "1s").ShouldNot(HaveOccurred())
		})

		It("frontend service should eventually have at least 5 message consumed", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				AppName: appName,
				Name:    lbwsServiceName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(svc.Routes)).To(Equal(1))
			route := svc.Routes[0]
			Expect(route.Environment).To(Equal(envName))
			Eventually(func() error {
				resp, err := http.Get(fmt.Sprintf("%s/count", route.URL))
				if err != nil {
					return err
				}
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("response status is %d and not %d", resp.StatusCode, http.StatusOK)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				fmt.Printf("response: %s\n", string(body))
				// The counter service add to the counter when the worker service processes a message
				count, _ := strconv.Atoi(string(body))
				if count < 5 {
					return fmt.Errorf("the counter is %v, but expected to be at least %v", count, 5)
				}
				return nil
			}, "100s", "10s").ShouldNot(HaveOccurred())
		})
	})
})
