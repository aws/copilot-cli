// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	cdkVersion              = "2.56.0"
	cdkConstructsMinVersion = "10.0.0"
	cdkTemplatesPath        = "overrides/cdk"
)

var (
	shortenServiceName = map[string]string{
		"ApiGatewayV2":           "apigwv2",
		"AppRunner":              "ar",
		"AutoScalingPlans":       "asgplans",
		"ApplicationAutoScaling": "appasg",
		"AutoScaling":            "asg",
		"CertificateManager":     "acm",
		"CloudFormation":         "cfn",
		"CloudFront":             "cf",
		"ServiceDiscovery":       "sd",
		"CloudWatch":             "cw",
		"CodeBuild":              "cb",
		"CodePipeline":           "cp",
		"DynamoDB":               "ddb",
		"ElasticLoadBalancingV2": "elbv2",
		"OpenSearchService":      "oss",
		"Route53":                "r53",
		"StepFunctions":          "sfn",
	}
)

// CFNResource represents a resource rendered in a CloudFormation template.
type CFNResource struct {
	Type      CFNType
	LogicalID string
}

type cfnResources []CFNResource

// UniqueTypes returns the list of unique CFN types.
func (rs cfnResources) UniqueTypes() []CFNType {
	var uniqueTypes []CFNType
	seen := make(map[CFNType]struct{})
	for _, r := range rs {
		if _, ok := seen[r.Type]; !ok {
			uniqueTypes = append(uniqueTypes, r.Type)
		}
		seen[r.Type] = struct{}{}
	}
	return uniqueTypes
}

// CFNType is a CloudFormation resource type such as "AWS::ECS::Service".
type CFNType string

// ImportName returns the name of the CDK package for the given CloudFormation type.
func (t CFNType) ImportName() string {
	parts := strings.Split(strings.ToLower(string(t)), "::")
	return fmt.Sprintf("aws_%s", parts[1])
}

// ImportShortRename returns a human-friendly shortened rename of the CDK package for the given CloudFormation type.
func (t CFNType) ImportShortRename() string {
	parts := strings.Split(string(t), "::")
	name := parts[1]

	if rename, ok := shortenServiceName[name]; ok {
		return rename
	}
	return strings.ToLower(name)
}

// L1ConstructName returns the name of the L1 construct representing the CloudFormation type.
func (t CFNType) L1ConstructName() string {
	parts := strings.Split(string(t), "::")
	return fmt.Sprintf("Cfn%s", parts[2])
}

// WalkOverridesCDKDir walks through the overrides/cdk templates and calls fn for each parsed template file.
func (t *Template) WalkOverridesCDKDir(resources []CFNResource, fn WalkDirFunc) error {
	type metadata struct {
		Version           string
		ConstructsVersion string
		Resources         cfnResources
	}
	return t.walkDir(cdkTemplatesPath, cdkTemplatesPath, metadata{
		Version:           cdkVersion,
		ConstructsVersion: cdkConstructsMinVersion,
		Resources:         resources,
	}, fn, WithFuncs(
		map[string]interface{}{
			// transform all the initial capital letters into lower letters.
			"camelCase": func(pascal string) string {
				firstSmall := len(pascal)
				for i, r := range pascal {
					if unicode.IsLower(r) {
						firstSmall = i
						break
					}
				}
				return strings.ToLower(pascal[:firstSmall]) + pascal[firstSmall:]
			},
		},
	))
}
