// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudfront_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var appName string
var bucketName string
var s3Client *s3.S3
var s3Manager *s3manager.Uploader
var staticPath string

const domainName = "copilot-e2e-tests.ecs.aws.dev"

func TestCloudFront(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudFront Suite")
}

var _ = BeforeSuite(func() {
	copilotCLI, err := client.NewCLI()
	Expect(err).NotTo(HaveOccurred())
	cli = copilotCLI
	appName = fmt.Sprintf("e2e-cloudfront-%d", time.Now().Unix())
	bucketName = appName
	err = os.Setenv("BUCKETNAME", bucketName)
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("DOMAINNAME", domainName)
	Expect(err).NotTo(HaveOccurred())
	staticPath = "static/index.html"
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	Expect(err).NotTo(HaveOccurred())
	s3Client = s3.New(sess)
	s3Manager = s3manager.NewUploader(sess)
})

var _ = AfterSuite(func() {
	_, err := cli.AppDelete()
	Expect(err).NotTo(HaveOccurred())

	// Empty and delete the S3 bucket.
	_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(staticPath),
	})
	Expect(err).NotTo(HaveOccurred())
	_, err = s3Client.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	Expect(err).NotTo(HaveOccurred())
})
