// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package multi_pipeline_test

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/copilot-cli/e2e/internal/client"
)

// Command-line tools.
var (
	copilot *client.CLI
	aws     *client.AWS
)

// Application identifiers.
var (
	appName          = fmt.Sprintf("e2e-multipipeline-%d", time.Now().Unix())
	testPipelineName = "my-pipeline-test"
	prodPipelineName = "my-pipeline-prod"
)

// CodeCommit credentials.
var (
	repoName          = appName
	repoURL           string
	codeCommitIAMUser = fmt.Sprintf("%s-cc", appName)
	codeCommitCreds   *client.IAMServiceCreds
)

func TestPipeline(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipeline Suite")
}

var _ = BeforeSuite(func() {
	cli, err := client.NewCLIWithDir(repoName)
	Expect(err).NotTo(HaveOccurred())
	copilot = cli
	aws = client.NewAWS()

	creds, err := aws.CreateCodeCommitIAMUser(codeCommitIAMUser)
	Expect(err).NotTo(HaveOccurred())
	codeCommitCreds = creds
})

var _ = AfterSuite(func() {
	_, err := copilot.AppDelete()
	_ = aws.DeleteCodeCommitRepo(appName)
	_ = aws.DeleteCodeCommitIAMUser(codeCommitIAMUser, codeCommitCreds.CredentialID)
	Expect(err).NotTo(HaveOccurred())
})
