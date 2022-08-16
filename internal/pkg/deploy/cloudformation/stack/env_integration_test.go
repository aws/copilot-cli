//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"os"
	"path/filepath"
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
		input          *deploy.CreateEnvironmentInput
		wantedFileName string
	}{
		"generate template with embedded manifest file with container insights and imported certificates and advanced access logs": {
			input: func() *deploy.CreateEnvironmentInput {
				rawMft := `name: test
type: Environment
# Create the public ALB with certificates attached.
cdn:
  certificate: viewer-cert
http:
  public:
    access_logs:
      bucket_name: accesslogsbucket
      prefix: accesslogsbucketprefix
    security_groups:
      ingress:
        restrict_to:
          cdn: true
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
				return &deploy.CreateEnvironmentInput{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					CIDRPrefixListIDs:    []string{"pl-mockid"},
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					CustomResourcesURLs: map[string]string{
						"CertificateValidationFunction": "https://mockbucket.s3-us-west-2.amazonaws.com/dns-cert-validator",
						"DNSDelegationFunction":         "https://mockbucket.s3-us-west-2.amazonaws.com/dns-delegation",
						"CustomDomainFunction":          "https://mockbucket.s3-us-west-2.amazonaws.com/custom-domain",
						"UniqueJSONValuesFunction":      "https://mockbucket.s3-us-west-2.amazonaws.com/unique-json-values",
					},
					Mft:    &mft,
					RawMft: []byte(rawMft),
				}
			}(),
			wantedFileName: "template-with-imported-certs-observability.yml",
		},
		"generate template with default access logs": {
			input: func() *deploy.CreateEnvironmentInput {
				rawMft := `name: test
type: Environment
http:
  public:
    access_logs: true`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &deploy.CreateEnvironmentInput{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					CustomResourcesURLs: map[string]string{
						"CertificateValidationFunction": "https://mockbucket.s3-us-west-2.amazonaws.com/dns-cert-validator",
						"DNSDelegationFunction":         "https://mockbucket.s3-us-west-2.amazonaws.com/dns-delegation",
						"CustomDomainFunction":          "https://mockbucket.s3-us-west-2.amazonaws.com/custom-domain",
					},
					Mft:    &mft,
					RawMft: []byte(rawMft),
				}
			}(),
			wantedFileName: "template-with-default-access-log-config.yml",
		},
		"generate template with embedded manifest file with custom security groups rules added by the customer": {
			input: func() *deploy.CreateEnvironmentInput {
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
				return &deploy.CreateEnvironmentInput{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					CustomResourcesURLs: map[string]string{
						"CertificateValidationFunction": "https://mockbucket.s3-us-west-2.amazonaws.com/dns-cert-validator",
						"DNSDelegationFunction":         "https://mockbucket.s3-us-west-2.amazonaws.com/dns-delegation",
						"CustomDomainFunction":          "https://mockbucket.s3-us-west-2.amazonaws.com/custom-domain",
					},
					AllowVPCIngress: true,
					Mft:             &mft,
					RawMft:          []byte(rawMft),
				}
			}(),

			wantedFileName: "template-with-custom-security-group.yml",
		},
		"generate template with embedded manifest file with empty security groups rules added by the customer": {
			input: func() *deploy.CreateEnvironmentInput {
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
security_group:
  ingress:
  egress:`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &deploy.CreateEnvironmentInput{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					CustomResourcesURLs: map[string]string{
						"CertificateValidationFunction": "https://mockbucket.s3-us-west-2.amazonaws.com/dns-cert-validator",
						"DNSDelegationFunction":         "https://mockbucket.s3-us-west-2.amazonaws.com/dns-delegation",
						"CustomDomainFunction":          "https://mockbucket.s3-us-west-2.amazonaws.com/custom-domain",
					},
					AllowVPCIngress: true,
					Mft:             &mft,
					RawMft:          []byte(rawMft),
				}
			}(),

			wantedFileName: "template-with-custom-empty-security-group.yml",
		},
		"generate template with custom resources": {
			input: func() *deploy.CreateEnvironmentInput {
				rawMft := `name: test
type: Environment`
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(rawMft), &mft)
				require.NoError(t, err)
				return &deploy.CreateEnvironmentInput{
					Version: "1.x",
					App: deploy.AppInformation{
						AccountPrincipalARN: "arn:aws:iam::000000000:root",
						Name:                "demo",
					},
					Name:                 "test",
					ArtifactBucketARN:    "arn:aws:s3:::mockbucket",
					ArtifactBucketKeyARN: "arn:aws:kms:us-west-2:000000000:key/1234abcd-12ab-34cd-56ef-1234567890ab",
					CustomResourcesURLs: map[string]string{
						"CertificateValidationFunction": "https://mockbucket.s3-us-west-2.amazonaws.com/dns-cert-validator",
						"DNSDelegationFunction":         "https://mockbucket.s3-us-west-2.amazonaws.com/dns-delegation",
						"CustomDomainFunction":          "https://mockbucket.s3-us-west-2.amazonaws.com/custom-domain",
					},
					Mft:    &mft,
					RawMft: []byte(rawMft),
				}
			}(),
			wantedFileName: "template-with-basic-manifest.yml",
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
			envStack := stack.NewEnvStackConfig(tc.input)
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
			require.Equal(t, wantedObj, actualObj)
		})
	}
}
