// +build integration
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/stretchr/testify/require"
)

func TestAutoscalingIntegration_Validate(t *testing.T) {
	path := filepath.Join("testdata", "autoscaling", manifestPath)
	wantedManifestBytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload(wantedManifestBytes)
	require.NoError(t, err)
	v, ok := mft.(*manifest.LoadBalancedWebService)
	require.Equal(t, ok, true)
	serializer, err := stack.NewHTTPSLoadBalancedWebService(v, env, appName, stack.RuntimeConfig{
		Image: &stack.ECRImage{
			RepoURL:  imageURL,
			ImageTag: imageTag,
		},
	})
	require.NoError(t, err)
	tpl, err := serializer.Template()
	require.NoError(t, err)
	sess, err := sessions.NewProvider().Default()
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
	manifestBytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload(manifestBytes)
	require.NoError(t, err)
	v, ok := mft.(*manifest.ScheduledJob)
	require.True(t, ok)
	serializer, err := stack.NewScheduledJob(v, env.Name, appName, stack.RuntimeConfig{})

	tpl, err := serializer.Template()
	require.NoError(t, err, "template should render")

	sess, err := sessions.NewProvider().Default()
	require.NoError(t, err)
	cfn := cloudformation.New(sess)

	t.Run("CF template should be valid", func(t *testing.T) {
		_, err := cfn.ValidateTemplate(&cloudformation.ValidateTemplateInput{
			TemplateBody: aws.String(tpl),
		})
		require.NoError(t, err)
	})
}
