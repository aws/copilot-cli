//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/stretchr/testify/require"
)

func TestEnvStack_Template(t *testing.T) {
	testCases := map[string]struct {
		input          *stack.EnvConfig
		wantedFileName string
	}{
		"generate template with embedded manifest file with container insights and cloudfront imported bucket and advanced access logs": {
			input: func() *stack.EnvConfig {
				rawMft := `name: test
type: Environment
# Create the public ALB with certificates attached.
cdn:
  certificate: viewer-cert
  static_assets:
    location: cf-s3-ecs-demo-bucket.s3.us-west-2.amazonaws.com
    alias: example.com
    path: static/*
http:
  public:
    ingress:
      cdn: true
      source_ips:
        - 1.1.1.1
        - 2.2.2.2
    access_logs:
      bucket_name: accesslogsbucket
      prefix: accesslogsbucketprefix
    certificates:
      - cert-1
      - cert-2
  private:
    security_groups:
      ingress:
        from_vpc: true
observability:
  container_insights: true # Enable container insights.`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &stack.EnvConfig{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					CIDRPrefixListIDs:    []string{"pl-mockid"},
					PublicALBSourceIPs:   []string{"1.1.1.1", "2.2.2.2"},
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					Mft:                  &mft,
					RawMft:               rawMft,
				}
			}(),
			wantedFileName: "template-with-cloudfront-observability.yml",
		},
		"generate template with default access logs": {
			input: func() *stack.EnvConfig {
				rawMft := `name: test
type: Environment
http:
  public:
    access_logs: true`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &stack.EnvConfig{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					Mft:                  &mft,
					RawMft:               rawMft,
				}
			}(),
			wantedFileName: "template-with-default-access-log-config.yml",
		},
		"generate template with embedded manifest file with custom security groups rules added by the customer": {
			input: func() *stack.EnvConfig {
				rawMft := `name: test
type: Environment
# Create the public ALB with certificates attached.
http:
  public:
    certificates:
      - cert-1
      - cert-2
observability:
  container_insights: true # Enable container insights.
network:
  vpc:
    security_group:
      ingress:
        - ip_protocol: tcp
          ports: 10
          cidr: 0.0.0.0
        - ip_protocol: tcp
          ports: 1-10
          cidr: 0.0.0.0
      egress:
        - ip_protocol: tcp
          ports: 0-65535
          cidr: 0.0.0.0`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &stack.EnvConfig{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					Mft:                  &mft,
					RawMft:               rawMft,
				}
			}(),

			wantedFileName: "template-with-custom-security-group.yml",
		},
		"generate template with embedded manifest file with imported certificates and SSL Policy and empty security groups rules added by the customer": {
			input: func() *stack.EnvConfig {
				rawMft := `name: test
type: Environment
# Create the public ALB with certificates attached.
http:
  public:
    certificates:
      - cert-1
      - cert-2
    ssl_policy: ELBSecurityPolicy-FS-1-1-2019-08
observability:
  container_insights: true # Enable container insights.
security_group:
  ingress:
  egress:`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &stack.EnvConfig{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					Mft:                  &mft,
					RawMft:               rawMft,
				}
			}(),

			wantedFileName: "template-with-imported-certs-sslpolicy-custom-empty-security-group.yml",
		},
		"generate template with custom resources": {
			input: func() *stack.EnvConfig {
				rawMft := `name: test
type: Environment`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &stack.EnvConfig{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					Mft:                  &mft,
					RawMft:               rawMft,
				}
			}(),
			wantedFileName: "template-with-basic-manifest.yml",
		},
		"generate template with default vpc and flowlogs is on": {
			input: func() *stack.EnvConfig {
				rawMft := `name: test
type: Environment
network:
  vpc:
    flow_logs:
     retention: 60`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &stack.EnvConfig{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					Mft:                  &mft,
					RawMft:               rawMft,
				}
			}(),
			wantedFileName: "template-with-defaultvpc-flowlogs.yml",
		},
		"generate template with imported vpc and flowlogs is on": {
			input: func() *stack.EnvConfig {
				rawMft := `name: test
type: Environment
network:
  vpc:
    id: 'vpc-12345'
    subnets:
      public:
        - id: 'subnet-11111'
        - id: 'subnet-22222'
      private:
        - id: 'subnet-33333'
        - id: 'subnet-44444'
    flow_logs: on`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &stack.EnvConfig{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					Mft:                  &mft,
					RawMft:               rawMft,
				}
			}(),
			wantedFileName: "template-with-importedvpc-flowlogs.yml",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			wanted, err := os.ReadFile(filepath.Join("testdata", "environments", tc.wantedFileName))
			require.NoError(t, err, "read wanted template")
			wantedObj := make(map[any]any)
			require.NoError(t, yaml.Unmarshal(wanted, wantedObj))

			// WHEN
			envStack, err := stack.NewEnvStackConfig(tc.input)
			require.NoError(t, err)
			actual, err := envStack.Template()
			require.NoError(t, err, "serialize template")
			actualObj := make(map[any]any)
			require.NoError(t, yaml.Unmarshal([]byte(actual), actualObj))
			actualMetadata := actualObj["Metadata"].(map[string]any) // We remove the Version from the expected template, as the latest env version always changes.
			delete(actualMetadata, "Version")
			// Strip new lines when comparing outputs.
			actualObj["Metadata"].(map[string]any)["Manifest"] = strings.TrimSpace(actualObj["Metadata"].(map[string]any)["Manifest"].(string))
			wantedObj["Metadata"].(map[string]any)["Manifest"] = strings.TrimSpace(wantedObj["Metadata"].(map[string]any)["Manifest"].(string))

			// THEN
			resetCustomResourceLocations(actualObj)
			compareStackTemplate(t, wantedObj, actualObj)
		})
	}
}

func TestEnvStack_Regression(t *testing.T) {
	testCases := map[string]struct {
		originalManifest *stack.EnvConfig
		newManifest      *stack.EnvConfig
	}{
		"should produce the same template after migrating load balancer ingress fields": {
			originalManifest: func() *stack.EnvConfig {
				rawMft := `name: test
type: Environment
# Create the public ALB with certificates attached.
cdn:
  certificate: viewer-cert
http:
  public:
    security_groups:
      ingress:
        restrict_to:
          cdn: true
    access_logs:
      bucket_name: accesslogsbucket
      prefix: accesslogsbucketprefix
    certificates:
      - cert-1
      - cert-2
  private:
    security_groups:
      ingress:
        from_vpc: true
observability:
  container_insights: true # Enable container insights.`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &stack.EnvConfig{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					CIDRPrefixListIDs:    []string{"pl-mockid"},
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					Mft:                  &mft,
					RawMft:               rawMft,
				}
			}(),
			newManifest: func() *stack.EnvConfig {
				rawMft := `name: test
type: Environment
# Create the public ALB with certificates attached.
cdn:
  certificate: viewer-cert
http:
  public:
    ingress:
      cdn: true
    access_logs:
      bucket_name: accesslogsbucket
      prefix: accesslogsbucketprefix
    certificates:
      - cert-1
      - cert-2
  private:
    ingress:
      vpc: true
observability:
  container_insights: true # Enable container insights.`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &stack.EnvConfig{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					CIDRPrefixListIDs:    []string{"pl-mockid"},
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					Mft:                  &mft,
					RawMft:               rawMft,
				}
			}(),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			originalStack, err := stack.NewEnvStackConfig(tc.originalManifest)
			require.NoError(t, err)
			originalTmpl, err := originalStack.Template()
			require.NoError(t, err, "should serialize the template given the original environment manifest")
			originalObj := make(map[any]any)
			require.NoError(t, yaml.Unmarshal([]byte(originalTmpl), originalObj))

			newStack, err := stack.NewEnvStackConfig(tc.newManifest)
			require.NoError(t, err)
			newTmpl, err := newStack.Template()
			require.NoError(t, err, "should serialize the template given a migrated environment manifest")
			newObj := make(map[any]any)
			require.NoError(t, yaml.Unmarshal([]byte(newTmpl), newObj))

			// Delete because manifest could be different.
			delete(originalObj["Metadata"].(map[string]any), "Manifest")
			delete(newObj["Metadata"].(map[string]any), "Manifest")

			resetCustomResourceLocations(originalObj)
			resetCustomResourceLocations(newObj)
			compareStackTemplate(t, originalObj, newObj)
		})
	}
}

func compareStackTemplate(t *testing.T, wantedObj, actualObj map[any]any) {
	actual, wanted := reflect.ValueOf(actualObj), reflect.ValueOf(wantedObj)
	compareStackTemplateSection(t, reflect.ValueOf("Description"), wanted, actual)
	compareStackTemplateSection(t, reflect.ValueOf("Metadata"), wanted, actual)
	compareStackTemplateSection(t, reflect.ValueOf("Parameters"), wanted, actual)
	compareStackTemplateSection(t, reflect.ValueOf("Conditions"), wanted, actual)
	compareStackTemplateSection(t, reflect.ValueOf("Outputs"), wanted, actual)
	// Compare each resource.
	actualResources, wantedResources := actual.MapIndex(reflect.ValueOf("Resources")).Elem(), wanted.MapIndex(reflect.ValueOf("Resources")).Elem()
	actualResourceNames, wantedResourceNames := actualResources.MapKeys(), wantedResources.MapKeys()
	for _, key := range actualResourceNames {
		compareStackTemplateSection(t, key, wantedResources, actualResources)
	}
	for _, key := range wantedResourceNames {
		compareStackTemplateSection(t, key, wantedResources, actualResources)
	}
}

func compareStackTemplateSection(t *testing.T, key, wanted, actual reflect.Value) {
	actualExist, wantedExist := actual.MapIndex(key).IsValid(), wanted.MapIndex(key).IsValid()
	if !actualExist && !wantedExist {
		return
	}
	require.True(t, actualExist,
		fmt.Sprintf("%q does not exist in the actual template", key.Interface()))
	require.True(t, wantedExist,
		fmt.Sprintf("%q does not exist in the expected template", key.Interface()))
	require.Equal(t, wanted.MapIndex(key).Interface(), actual.MapIndex(key).Interface(),
		fmt.Sprintf("Comparing %q", key.Interface()))
}

func resetCustomResourceLocations(template map[any]any) {
	resources := template["Resources"].(map[string]any)
	functions := []string{
		"EnvControllerFunction", "DynamicDesiredCountFunction", "BacklogPerTaskCalculatorFunction",
		"RulePriorityFunction", "NLBCustomDomainFunction", "NLBCertValidatorFunction",
		"CustomDomainFunction", "CertificateValidationFunction", "DNSDelegationFunction",
		"CertificateReplicatorFunction", "UniqueJSONValuesFunction", "TriggerStateMachineFunction", "BucketCleanerFunction",
	}
	for _, fnName := range functions {
		resource, ok := resources[fnName]
		if !ok {
			continue
		}
		fn := resource.(map[string]any)
		props := fn["Properties"].(map[string]any)
		delete(props, "Code")
	}
}
