// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package grpc_svc_app_test

import (
	"crypto/tls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	initErr error
)

var _ = Describe("gRPC Service App", func() {
	const domainName = "copilot-e2e-tests.ecs.aws.dev"

	Context("when creating a new app", func() {
		BeforeAll(func() {
			_, initErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
				Domain:  domainName,
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
			Expect(appShowOutput.URI).To(Equal(domainName))
		})
	})

	Context("when creating a new environment", func() {
		It("env init should succeed for creating the test environment", func() {
			_, envInitErr := cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: "default",
				Prod:    false,
			})
			Expect(envInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when init Load Balanced gRPC Web Services", func() {

		It("svc init should succeed for creating the grpc service", func() {
			_, svcInitErr := cli.SvcInit(&client.SvcInitRequest{
				Name:       "grpc",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./grpc/Dockerfile",
				SvcPort:    "50051",
			})
			Expect(svcInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying a Load Balanced Web Service", func() {
		var deployErr error

		It("deploy grpc to test should succeed", func() {
			_, deployErr = cli.SvcDeploy(&client.SvcDeployInput{
				Name:     "grpc",
				EnvName:  "test",
				ImageTag: "grpc",
			})
			Expect(deployErr).NotTo(HaveOccurred())
		})

		It("svc show should contain the expected domain and the request should succeed", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				Name:    "grpc",
				AppName: appName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))

			wantedURLs := map[string]string{
				"test": "https://test.copilot-e2e-tests.ecs.aws.dev",
			}
			for _, route := range svc.Routes {
				// Validate route has the expected HTTPS endpoint.
				Expect(route.URL).To(Equal(wantedURLs[route.Environment]))

				// Make sure error response is nil.
				tlsCredentials := credentials.NewTLS(&tls.Config{})
				conn, err := grpc.Dial(route.URL, grpc.WithTransportCredentials(tlsCredentials))
				Expect(err).NotTo(HaveOccurred())
				defer conn.Close()
			}
		})
	})
})
