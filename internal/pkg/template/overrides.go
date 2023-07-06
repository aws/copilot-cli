// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	cdkVersion              = "2.56.0"
	cdkConstructsMinVersion = "10.0.0"
	cdkTemplatesPath        = "overrides/cdk"

	yamlPatchTemplatesPath = "overrides/yamlpatch"
)

var (
	cdkAliasForService = map[string]string{
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

// CDKImport is the interface to import a CDK package.
type CDKImport interface {
	ImportName() string
	ImportShortRename() string
}

type cfnResources []CFNResource

// Imports returns a list of CDK imports for a given list of CloudFormation resources.
func (rs cfnResources) Imports() []CDKImport {
	// Find a unique CFN type per service.
	// For example, given "AWS::ECS::Service" and "AWS::ECS::TaskDef" we want to retain only one of them.
	seen := make(map[string]CFNType)
	for _, r := range rs {
		serviceName := strings.Split(strings.ToLower(string(r.Type)), "::")[1]
		if _, ok := seen[serviceName]; ok {
			continue
		}
		seen[serviceName] = r.Type
	}

	imports := make([]CDKImport, len(seen))
	i := 0
	for _, resourceType := range seen {
		imports[i] = resourceType
		i += 1
	}
	sort.Slice(imports, func(i, j int) bool { // Ensure the output is deterministic for unit tests.
		return imports[i].ImportShortRename() < imports[j].ImportShortRename()
	})
	return imports
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

	if rename, ok := cdkAliasForService[name]; ok {
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
func (t *Template) WalkOverridesCDKDir(resources []CFNResource, fn WalkDirFunc, requiresEnv bool) error {
	type metadata struct {
		Version           string
		ConstructsVersion string
		Resources         cfnResources
		RequiresEnv       bool
	}
	return t.walkDir(cdkTemplatesPath, cdkTemplatesPath, metadata{
		Version:           cdkVersion,
		ConstructsVersion: cdkConstructsMinVersion,
		Resources:         resources,
		RequiresEnv:       requiresEnv,
	}, fn, WithFuncs(
		map[string]interface{}{
			// transform all the initial capital letters into lower letters.
			"lowerInitialLetters": func(serviceName string) string {
				firstSmall := len(serviceName)
				for i, r := range serviceName {
					if unicode.IsLower(r) {
						firstSmall = i
						break
					}
				}
				return strings.ToLower(serviceName[:firstSmall]) + serviceName[firstSmall:]
			},
		},
	))
}

// WalkOverridesPatchDir walks through the overrides/yamlpatch templates and calls fn for each parsed template file.
func (t *Template) WalkOverridesPatchDir(fn WalkDirFunc) error {
	return t.walkDir(yamlPatchTemplatesPath, yamlPatchTemplatesPath, struct{}{}, fn)
}
