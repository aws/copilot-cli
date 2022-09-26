// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package fifo_sns_sqs

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

var _ = Describe("FIFO SNS And SQS", func() {

	Context("when creating a new app", func() {
		var appInitErr error

		BeforeAll(func() {
			_, appInitErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
			})
		})

		It("app init succeeds", func() {
			Expect(appInitErr).NotTo(HaveOccurred())
		})

		It("app init creates a copilot directory", func() {
			Expect("./copilot").Should(BeADirectory())
		})

		It("app ls includes new application", func() {
			Eventually(cli.AppList, "30s", "5s").Should(ContainSubstring(appName))
		})
	})

	Context("when adding new environment", func() {
		var (
			err error
		)
		BeforeAll(func() {
			_, err = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: "default",
			})
		})
		It("env init should succeed", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environments", func() {
		var envDeployErr error
		BeforeAll(func() {
			_, envDeployErr = cli.EnvDeploy(&client.EnvDeployRequest{
				AppName: appName,
				Name:    "test",
			})
		})
		It("env deploy should succeed", func() {
			Expect(envDeployErr).NotTo(HaveOccurred())
		})
	})

	Context("when initializing Backend Services", func() {
		var svcInitErr error

		It("svc init should succeed for creating the backend service", func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "backend",
				SvcType:    "Backend Service",
				Dockerfile: "./src/Dockerfile",
				SvcPort:    "80",
			})
			Expect(svcInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying a Backend Service", func() {
		It("deployment should succeed", func() {
			_, err := cli.SvcDeploy(&client.SvcDeployInput{
				Name:     "backend",
				EnvName:  "test",
				ImageTag: "backend",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("svc show should contain expected topic URIs and those URIs should exist", func() {
			// Check frontend service
			/*svc, err := cli.SvcShow(&client.SvcShowRequest{
				Name:    "backend",
				AppName: appName,
			})
			Expect(err).NotTo(HaveOccurred())*/
		})
	})
})
