// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package isolated_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var aws *client.AWS
var appName string
var vpcStackName string
var vpcStackTemplatePath string
var vpcImport client.EnvInitRequestVPCImport
var timeNow = time.Now().Unix()

const svcName = "backend"
const envName = "test"

/**
The Isolated Suite creates an environment with an imported VPC with only
private subnets, deploys a backend service to it, and then tears it down.
*/
func Test_Isolated(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Isolated Suite")
}

var _ = BeforeSuite(func() {
	vpcStackName = fmt.Sprintf("e2e-isolated-vpc-stack-%d", timeNow)
	vpcStackTemplatePath = "file://vpc.yml"
	copilot, err := client.NewCLI()
	Expect(err).NotTo(HaveOccurred())
	cli = copilot
	aws = client.NewAWS()
	appName = fmt.Sprintf("e2e-isolated-%d", timeNow)
	// Create the VPC stack.
	err = aws.CreateStack(vpcStackName, vpcStackTemplatePath)
	Expect(err).NotTo(HaveOccurred())
	err = aws.WaitStackCreateComplete(vpcStackName)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	_, deleteAppErr := cli.AppDelete()
	deleteVPCErr := deleteVPCAndWait()
	Expect(deleteAppErr).NotTo(HaveOccurred())
	Expect(deleteVPCErr).NotTo(HaveOccurred())
})

func deleteVPCAndWait() error {
	err := aws.DeleteStack(vpcStackName)
	if err != nil {
		return err
	}
	return aws.WaitStackDeleteComplete(vpcStackName)
}
