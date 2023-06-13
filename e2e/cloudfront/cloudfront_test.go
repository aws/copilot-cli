// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudfront_test

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/copilot-cli/e2e/internal/client"
)

var _ = Describe("CloudFront", func() {
	Context("when creating an S3 bucket and upload static files", Ordered, func() {
		It("bucket creation should succeed", func() {
			_, err := s3Client.CreateBucket(&s3.CreateBucketInput{
				Bucket:          aws.String(bucketName),
				ObjectOwnership: aws.String(s3.ObjectOwnershipBucketOwnerEnforced),
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("upload should succeed", func() {
			_, err := s3Manager.Upload(&s3manager.UploadInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(staticPath),
				Body:   bytes.NewBufferString("hello static"),
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when creating a new app", Ordered, func() {
		var appInitErr error

		BeforeAll(func() {
			_, appInitErr = cli.AppInit(&client.AppInitRequest{
				AppName: appName,
				Domain:  domainName,
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

		It("app show includes domain name", func() {
			appShowOutput, err := cli.AppShow(appName)
			Expect(err).NotTo(HaveOccurred())
			Expect(appShowOutput.Name).To(Equal(appName))
			Expect(appShowOutput.URI).To(Equal(domainName))
		})
	})

	Context("when adding new environment", Ordered, func() {
		var err error
		BeforeAll(func() {
			_, err = cli.EnvInit(&client.EnvInitRequest{
				AppName: appName,
				EnvName: "test",
				Profile: "test",
			})
		})
		It("env init should succeed", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when deploying the environments", Ordered, func() {
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

	Context("when initializing Load Balanced Web Services", Ordered, func() {
		var svcInitErr error
		BeforeAll(func() {
			_, svcInitErr = cli.SvcInit(&client.SvcInitRequest{
				Name:       "frontend",
				SvcType:    "Load Balanced Web Service",
				Dockerfile: "./src/Dockerfile",
				SvcPort:    "80",
			})
		})
		It("svc init should succeed for creating the frontend service", func() {
			Expect(svcInitErr).NotTo(HaveOccurred())
		})
	})

	Context("when deploying a Load Balanced Web Service", func() {
		It("deployment should succeed", func() {
			_, err := cli.SvcDeploy(&client.SvcDeployInput{
				Name:     "frontend",
				EnvName:  "test",
				ImageTag: "frontend",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("updating s3 bucket policy should succeed", func() {
			out, err := cli.EnvShow(&client.EnvShowRequest{
				AppName: appName,
				EnvName: "test",
			})
			Expect(err).NotTo(HaveOccurred())
			var accountID string
			var cfDistID string
			for _, resource := range out.Resources {
				switch resource["logicalID"] {
				case "PublicLoadBalancer":
					parsed, err := arn.Parse(resource["physicalID"])
					Expect(err).NotTo(HaveOccurred())
					accountID = parsed.AccountID
				case "CloudFrontDistribution":
					cfDistID = resource["physicalID"]
				}
			}
			Expect(accountID).ToNot(BeEmpty())
			Expect(cfDistID).ToNot(BeEmpty())
			_, err = s3Client.PutBucketPolicy(&s3.PutBucketPolicyInput{
				Bucket: aws.String(bucketName),
				Policy: aws.String(fmt.Sprintf(`{
	"Version": "2012-10-17",
	"Statement": [
			{
					"Sid": "AllowCloudFrontServicePrincipalReadOnly",
					"Effect": "Allow",
					"Principal": {
							"Service": "cloudfront.amazonaws.com"
					},
					"Action": "s3:GetObject",
					"Resource": "arn:aws:s3:::%s/static/*",
					"Condition": {
							"StringEquals": {
									"AWS:SourceArn": "arn:aws:cloudfront::%s:distribution/%s"
							}
					}
			}
	]
}
`, bucketName, accountID, cfDistID)),
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("svc show should contain the expected domain and the request should succeed", func() {
			svc, err := cli.SvcShow(&client.SvcShowRequest{
				Name:    "frontend",
				AppName: appName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(svc.Routes)).To(Equal(1))
			route := svc.Routes[0]
			wantedURLs := map[string]string{
				"test": fmt.Sprintf("https://frontend-%d.%s", timeNow, domainName),
			}
			// Validate route has the expected HTTPS endpoint.
			Expect(route.URL).To(Equal(wantedURLs[route.Environment]))

			// Make sure the service response is OK.
			var resp *http.Response
			var fetchErr error
			Eventually(func() (int, error) {
				resp, fetchErr = http.Get(route.URL)
				return resp.StatusCode, fetchErr
			}, "60s", "1s").Should(Equal(200))
			// HTTP should work.
			Eventually(func() (int, error) {
				resp, fetchErr = http.Get(strings.Replace(route.URL, "https", "http", 1))
				return resp.StatusCode, fetchErr
			}, "60s", "1s").Should(Equal(200))
			// Static assets should be accessible.
			Eventually(func() (int, error) {
				resp, fetchErr = http.Get(fmt.Sprintf("%s/%s", route.URL, staticPath))
				return resp.StatusCode, fetchErr
			}, "60s", "1s").Should(Equal(200))
		})
	})
})
