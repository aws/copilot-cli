//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/copilot-cli/internal/pkg/template"

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
		"generate template with embedded manifest file with container insights and imported certificates": {
			input: func() *deploy.CreateEnvironmentInput {
				var mft manifest.Environment
				err := yaml.Unmarshal([]byte(`
name: test
type: Environment
# Create the public ALB with certificates attached.
# All these comments should be deleted.
http:
  public:
    certificates:
      - cert-1
      - cert-2
observability:
    container_insights: true # Enable container insights.
`), &mft)
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
						template.DNSCertValidatorFileName: "https://mockbucket.s3-us-west-2.amazonaws.com/dns-cert-validator",
						template.DNSDelegationFileName:    "https://mockbucket.s3-us-west-2.amazonaws.com/dns-delegation",
						template.CustomDomainFileName:     "https://mockbucket.s3-us-west-2.amazonaws.com/custom-domain",
					},
					Mft: &mft,
				}
			}(),
			wantedFileName: "template-with-imported-certs-observability.yml",
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

			// THEN
			require.Equal(t, wantedObj, actualObj)
		})
	}
}
