// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTemplateDescriptions(t *testing.T) {
	testCases := map[string]struct {
		testFile           string
		wantedDescriptions map[string]string
	}{
		"parse environment template": {
			testFile: "env.yaml",
			wantedDescriptions: map[string]string{
				"VPC":                "Virtual private cloud on 2 availability zones to control network boundaries.",
				"PublicRouteTable":   "Routing table for services to talk with each other.",
				"InternetGateway":    "Internet gateway to connect the network to the internet.",
				"PublicSubnet1":      "A public subnet in your first AZ for internet facing services.",
				"PublicSubnet2":      "A public subnet in your second AZ for internet facing services.",
				"Cluster":            "An ECS Cluster to hold your services.",
				"PublicLoadBalancer": "An application load balancer to distribute traffic to your Load Balanced Web Services.",
			},
		},
		"parse load balanced web service template": {
			testFile: "lb-web-svc.yaml",
			wantedDescriptions: map[string]string{
				"LogGroup":            "A CloudWatch log group to store your logs.",
				"TaskDefinition":      "An ECS TaskDefinition where your containers are defined.",
				"DiscoveryService":    "Service discovery to communicate with other services in your VPC.",
				"EnvControllerAction": "Updating your environment to enable load balancers.",
				"Service":             "An ECS service to run and maintain your tasks.",
				"TargetGroup":         "A target group to connect your service to the load balancer.",
				"AddonsStack":         "An addons stack for your additional AWS resources.",
				"TaskRole":            "A task role to manage permissions for your containers.",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			body, err := os.ReadFile(filepath.Join("testdata", "parse", tc.testFile))
			require.NoError(t, err, tc.testFile, "unexpected error while reading testdata file")

			// WHEN
			descriptions, err := ParseTemplateDescriptions(string(body))

			// THEN
			require.NoError(t, err, "parsing the cloudformation template should not error")
			require.Equal(t, tc.wantedDescriptions, descriptions, "descriptions should match")
		})
	}
}
