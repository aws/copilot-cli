// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"testing"

	"github.com/spf13/afero"

	"github.com/stretchr/testify/require"
)

func TestCFNType_ImportName(t *testing.T) {
	require.Equal(t, "aws_autoscaling", CFNType("AWS::AutoScaling::AutoScalingGroup").ImportName())
}

func TestCFNType_ImportShortRename(t *testing.T) {
	testCases := []struct {
		in     string
		wanted string
	}{
		{
			in:     "AWS::AutoScaling::AutoScalingGroup",
			wanted: "autoscaling",
		},
		{
			in:     "AWS::Logs::LogGroup",
			wanted: "logs",
		},
		{
			in:     "AWS::ECS::Service",
			wanted: "ecs",
		},
		{
			in:     "AWS::DynamoDB::Table",
			wanted: "ddb",
		},
		{
			in:     "AWS::ApiGatewayV2::Api",
			wanted: "agv2",
		},
		{
			in:     "AWS::EC2::CapacityReservation",
			wanted: "ec2",
		},
		{
			in:     "AWS::ElasticLoadBalancingV2::Listener",
			wanted: "elbv2",
		},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.wanted, CFNType(tc.in).ImportShortRename(), "unexpected short name for %q", tc.in)
	}
}

func TestCFNType_L1ConstructName(t *testing.T) {
	require.Equal(t, "CfnAutoScalingGroup", CFNType("AWS::AutoScaling::AutoScalingGroup").L1ConstructName())
}

func TestCfnResources_Types(t *testing.T) {
	require.ElementsMatch(t, []CFNType{
		"AWS::ECR::Repository",
		"AWS::ECS::Cluster",
		"AWS::IAM::Role",
	}, cfnResources([]CFNResource{
		{
			Type: "AWS::IAM::Role",
		},
		{
			Type: "AWS::ECS::Cluster",
		},
		{
			Type: "AWS::IAM::Role",
		},
		{
			Type: "AWS::ECR::Repository",
		},
		{
			Type: "AWS::ECR::Repository",
		},
	}).Types())
}

func TestTemplate_WalkOverridesCDKDir(t *testing.T) {
	// GIVEN
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll("templates/overrides/cdk/bin", 0755)
	_ = afero.WriteFile(fs, "templates/overrides/cdk/bin/app.js", []byte(`const app = new cdk.App();`), 0644)
	_ = afero.WriteFile(fs, "templates/overrides/cdk/package.json", []byte(`{
 "devDependencies": {
   "aws-cdk": "{{.Version}}",
   "ts-node": "^10.9.1",
   "typescript": "~4.9.4"
 },
 "dependencies": {
   "aws-cdk-lib": "{{.Version}}",
   "constructs": "^{{.ConstructsVersion}}",
   "source-map-support": "^0.5.21"
 }
}`), 0644)
	_ = afero.WriteFile(fs, "templates/overrides/cdk/stack.ts", []byte(`{{- range $resourceType := .Resources.Types }}
import { {{$resourceType.ImportName}} as {{$resourceType.ImportShortRename}} } from 'aws-cdk-lib';
{{- end }}
{{range $resource := .Resources}}
transform{{$resource.LogicalID}}() {
	const {{camelCase $resource.LogicalID}} = this.template.getResource("{{$resource.LogicalID}}") as {{$resource.Type.ImportShortRename}}.{{$resource.Type.L1ConstructName}};
}
{{end}}
`), 0644)

	tpl := &Template{
		fs: &mockFS{Fs: fs},
	}

	input := []CFNResource{
		{
			Type:      "AWS::ECS::Service",
			LogicalID: "Service",
		},
		{
			Type:      "AWS::ElasticLoadBalancingV2::ListenerRule",
			LogicalID: "HTTPListenerRuleWithDomain",
		},
		{
			Type:      "AWS::ElasticLoadBalancingV2::ListenerRule",
			LogicalID: "HTTPListenerRule",
		},
	}

	// WHEN
	walked := map[string]bool{
		"package.json": false,
		"stack.ts":     false,
		"bin/app.js":   false,
	}
	err := tpl.WalkOverridesCDKDir(input, func(name string, content *Content) error {
		switch name {
		case "package.json":
			walked["package.json"] = true
			require.Equal(t, `{
 "devDependencies": {
   "aws-cdk": "2.56.0",
   "ts-node": "^10.9.1",
   "typescript": "~4.9.4"
 },
 "dependencies": {
   "aws-cdk-lib": "2.56.0",
   "constructs": "^10.0.0",
   "source-map-support": "^0.5.21"
 }
}`, content.String())
		case "stack.ts":
			walked["stack.ts"] = true
			require.Equal(t, `
import { aws_ecs as ecs } from 'aws-cdk-lib';
import { aws_elasticloadbalancingv2 as elbv2 } from 'aws-cdk-lib';

transformService() {
	const service = this.template.getResource("Service") as ecs.CfnService;
}

transformHTTPListenerRuleWithDomain() {
	const httplistenerRuleWithDomain = this.template.getResource("HTTPListenerRuleWithDomain") as elbv2.CfnListenerRule;
}

transformHTTPListenerRule() {
	const httplistenerRule = this.template.getResource("HTTPListenerRule") as elbv2.CfnListenerRule;
}

`, content.String())
		case "bin/app.js":
			walked["bin/app.js"] = true
			require.Equal(t, "const app = new cdk.App();", content.String())
		}
		return nil
	})

	// THEN
	require.NoError(t, err)
	for name, ok := range walked {
		if !ok {
			require.FailNowf(t, "missing walk file", "file %q was not walked", name)
		}
	}
}
