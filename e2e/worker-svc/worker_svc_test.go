// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package worker_svc_test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

var (
	initErr error
)

var _ = Describe("Worker Service App", func() {
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

		It("app ls includes new application", func() {
			Eventually(cli.AppList, "30s", "5s").Should(ContainSubstring(appName))
		})

		It("app show includes app name", func() {
			appShowOutput, err := cli.AppShow(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(appShowOutput.Name).To(Equal(appName))
			Expect(appShowOutput.URI).To(BeEmpty())
		})
	})

	Context("when creating a new environment", func() {
		var (
			testEnvInitErr error
		)
		BeforeAll(func() {
			_, testEnvInitErr = cli.EnvInit(&client.EnvInitRequest{
				AppName:       appName,
				EnvName:       envName,
				Profile:       "default",
				Prod:          false,
				CustomizedEnv: false,
			})
		})

		It("env init should succeed", func() {
			Expect(testEnvInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when adding a publishing service", func() {
		var (
			publisherInitErr error
			workerInitErr    error
		)

		BeforeAll(func() {
			_, publisherInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       publisherName,
				SvcType:    "Backend Service",
				Dockerfile: "./publisher/Dockerfile",
			})
			_, workerInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       workerName,
				SvcType:    "Worker Service",
				Dockerfile: "./worker/Dockerfile",
			})
		})

		It("svc init should succeed", func() {
			Expect(publisherInitErr).NotTo(HaveOccurred())
			Expect(workerInitErr).NotTo(HaveOccurred())
		})

		It("svc init should create svc manifest", func() {
			Expect(fmt.Sprintf("./copilot/%s/manifest.yml", publisherName)).Should(BeAnExistingFile())
			Expect(fmt.Sprintf("./copilot/%s/manifest.yml", workerName)).Should(BeAnExistingFile())
		})

		It("svc ls should list the svc", func() {
			svcList, svcListError := cli.SvcList(appName)
			Expect(svcListError).NotTo(HaveOccurred())
			Expect(len(svcList.Services)).To(Equal(2))

			svcsByName := map[string]client.WkldDescription{}
			for _, svc := range svcList.Services {
				svcsByName[svc.Name] = svc
			}

			for _, svc := range []string{publisherName, workerName} {
				Expect(svcsByName[svc].Name).To(Equal(svc))
				Expect(svcsByName[svc].AppName).To(Equal(appName))
			}
		})

		It("svc package should output a cloudformation template and params file", func() {
			_, svcPackageError := cli.SvcPackage(&client.PackageInput{
				Name:    workerName,
				AppName: appName,
				Env:     envName,
				Dir:     "infrastructure",
				Tag:     "gallopinggurdey",
			})
			Expect(svcPackageError).NotTo(HaveOccurred())
			Expect(fmt.Sprintf("infrastructure/%s-test.stack.yml", workerName)).To(BeAnExistingFile())
			Expect(fmt.Sprintf("infrastructure/%s-test.params.json", workerName)).To(BeAnExistingFile())
		})
	})

	Context("When deploying services", func() {
		var (
			publisherDeployErr error
			workerDeployErr    error
		)

		BeforeAll(func() {
			_, publisherDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:    publisherName,
				EnvName: envName,
			})
			_, workerDeployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:    workerName,
				EnvName: envName,
			})
		})

		It("svc deploy should succeed", func() {
			Expect(publisherDeployErr).NotTo(HaveOccurred())
			Expect(workerDeployErr).NotTo(HaveOccurred())
		})

		It("publisher should have topic ARNs in its env vars", func() {
			svcShowResult, svcShowErr := cli.SvcShow(&client.SvcShowRequest{
				Name:    publisherName,
				AppName: appName,
			})
			// Svc show should succeed.
			Expect(svcShowErr).NotTo(HaveOccurred())
			// Service should contain environment variable
			testEnvVariables := make(map[string]string)
			for _, v := range svcShowResult.Variables {
				if v.Environment == envName {
					testEnvVariables[v.Name] = v.Value
				}
			}
			Expect(testEnvVariables).To(HaveKey("COPILOT_SNS_TOPIC_ARNS"))
			variableValue := []byte(testEnvVariables["COPILOT_SNS_TOPIC_ARNS"])
			var topicARNs []string
			err := json.Unmarshal(variableValue, &topicARNs)
			Expect(err).NotTo(HaveOccurred())
			Expect(variableValue).To(HaveLen(2))
		})

	})
})

/*
5. That the worker has both a service level and topic-level queue.
7. That the DLQ is created.
8. That the worker is able to pull messages from the queue (by checking logs for magic phrases which appear in messages).
9. That poison pill messages are routed to the DLQ.
*/
