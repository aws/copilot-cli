// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package customized_env_test

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
var vpcConfig client.EnvInitRequestVPCConfig

/**
The Customized Env Suite creates multiple environments with customized resources,
deploys a service to it, and then
tears it down.
*/
func Test_Customized_Env(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Customized Env Svc Suite")
}

var _ = BeforeSuite(func() {
	vpcStackName = fmt.Sprintf("e2e-customizedenv-vpc-stack-%d", time.Now().Unix())
	vpcStackTemplatePath = "file://vpc.yml"
	ecsCli, err := client.NewCLI()
	Expect(err).NotTo(HaveOccurred())
	cli = ecsCli
	aws = client.NewAWS()
	appName = fmt.Sprintf("e2e-customizedenv-%d", time.Now().Unix())
	vpcConfig = client.EnvInitRequestVPCConfig{
		CIDR:               "10.1.0.0/16",
		PrivateSubnetCIDRs: "10.1.2.0/24,10.1.3.0/24",
		PublicSubnetCIDRs:  "10.1.0.0/24,10.1.1.0/24",
	}
})

var _ = AfterSuite(func() {
	_, appDeleteErr := cli.AppDelete()
	// Delete VPC stack.
	vpcDeleteErr := aws.DeleteStack(vpcStackName)
	Expect(appDeleteErr).NotTo(HaveOccurred())
	Expect(vpcDeleteErr).NotTo(HaveOccurred())
})
