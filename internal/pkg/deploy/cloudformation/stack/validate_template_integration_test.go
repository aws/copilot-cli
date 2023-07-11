//go:build integration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestAutoscalingIntegration_Validate(t *testing.T) {
	path := filepath.Join("testdata", "stacklocal", autoScalingManifestPath)
	wantedManifestBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload(wantedManifestBytes)
	require.NoError(t, err)
	content := mft.Manifest()
	v, ok := content.(*manifest.LoadBalancedWebService)
	require.Equal(t, ok, true)

	// Create in-memory mock file system.
	wd, err := os.Getwd()
	require.NoError(t, err)
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", wd), 0755)
	_ = afero.WriteFile(fs, fmt.Sprintf("%s/copilot/.workspace", wd), []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
	require.NoError(t, err)
	ws, err := workspace.Use(fs)
	require.NoError(t, err)

	_, err = addon.ParseFromWorkload(aws.StringValue(v.Name), ws)
	var notFound *addon.ErrAddonsNotFound
	require.ErrorAs(t, err, &notFound)

	serializer, err := stack.NewLoadBalancedWebService(stack.LoadBalancedWebServiceConfig{
		App: &config.Application{Name: appName},
		EnvManifest: &manifest.Environment{
			Workload: manifest.Workload{
				Name: &envName,
			},
		},
		Manifest: v,
		RuntimeConfig: stack.RuntimeConfig{
			PushedImages: map[string]stack.ECRImage{
				aws.StringValue(v.Name): {
					RepoURL:  imageURL,
					ImageTag: imageTag,
				},
			},
			ServiceDiscoveryEndpoint: "test.app.local",
			CustomResourcesURL: map[string]string{
				"EnvControllerFunction":       "https://my-bucket.s3.us-west-2.amazonaws.com/code.zip",
				"DynamicDesiredCountFunction": "https://my-bucket.s3.us-west-2.amazonaws.com/code.zip",
				"RulePriorityFunction":        "https://my-bucket.s3.us-west-2.amazonaws.com/code.zip",
			},
			EnvVersion: "v1.42.0",
			Version:    "v1.29.0",
		},
	})
	require.NoError(t, err)
	tpl, err := serializer.Template()
	require.NoError(t, err)
	sess, err := sessions.ImmutableProvider().Default()
	require.NoError(t, err)
	cfn := cloudformation.New(sess)

	t.Run("CloudFormation template must be valid", func(t *testing.T) {
		_, err := cfn.ValidateTemplate(&cloudformation.ValidateTemplateInput{
			TemplateBody: aws.String(tpl),
		})
		require.NoError(t, err)
	})
}

func TestScheduledJob_Validate(t *testing.T) {
	path := filepath.Join("testdata", "workloads", jobManifestPath)
	manifestBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload(manifestBytes)
	require.NoError(t, err)
	content := mft.Manifest()
	v, ok := content.(*manifest.ScheduledJob)
	require.True(t, ok)

	// Create in-memory mock file system.
	wd, err := os.Getwd()
	require.NoError(t, err)
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", wd), 0755)
	_ = afero.WriteFile(fs, fmt.Sprintf("%s/copilot/.workspace", wd), []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
	require.NoError(t, err)
	ws, err := workspace.Use(fs)
	require.NoError(t, err)

	_, err = addon.ParseFromWorkload(aws.StringValue(v.Name), ws)
	var notFound *addon.ErrAddonsNotFound
	require.ErrorAs(t, err, &notFound)

	serializer, err := stack.NewScheduledJob(stack.ScheduledJobConfig{
		App: &config.Application{
			Name: appName,
		},
		Env:      envName,
		Manifest: v,
		RuntimeConfig: stack.RuntimeConfig{
			ServiceDiscoveryEndpoint: "test.app.local",
			CustomResourcesURL: map[string]string{
				"EnvControllerFunction": "https://my-bucket.s3.us-west-2.amazonaws.com/code.zip",
			},
			AccountID:  "123456789123",
			Region:     "us-west-2",
			EnvVersion: "v1.42.0",
			Version:    "v1.29.0",
		},
	})

	tpl, err := serializer.Template()
	require.NoError(t, err, "template should render")

	sess, err := sessions.ImmutableProvider().Default()
	require.NoError(t, err)
	cfn := cloudformation.New(sess)

	t.Run("CF template should be valid", func(t *testing.T) {
		_, err := cfn.ValidateTemplate(&cloudformation.ValidateTemplateInput{
			TemplateBody: aws.String(tpl),
		})
		require.NoError(t, err)
	})
}
