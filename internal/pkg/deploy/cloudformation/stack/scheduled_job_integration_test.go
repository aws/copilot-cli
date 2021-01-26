// +build integration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/stretchr/testify/require"
)

const (
	jobManifestPath = "job-manifest.yml"
	jobStackPath    = "job-test.stack.yml"
	jobParamsPath   = "job-test.params.json"
)

func TestScheduledJob_Template(t *testing.T) {
	path := filepath.Join("testdata", "workloads", jobManifestPath)
	manifestBytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload(manifestBytes)
	require.NoError(t, err)
	v, ok := mft.(*manifest.ScheduledJob)
	require.True(t, ok)
	serializer, err := stack.NewScheduledJob(v, envName, appName, stack.RuntimeConfig{})

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

	t.Run("CF Template should be equal", func(t *testing.T) {
		actualBytes := []byte(tpl)
		mActual := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(actualBytes, mActual))

		expected, err := ioutil.ReadFile(filepath.Join("testdata", "workloads", jobStackPath))
		require.NoError(t, err, "should be able to read expected bytes")
		expectedBytes := []byte(expected)
		mExpected := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(expectedBytes, mExpected))
		require.Equal(t, mExpected, mActual)
	})

	t.Run("Parameter values should render properly", func(t *testing.T) {
		actualParams, err := serializer.SerializedParameters()
		require.NoError(t, err)

		path := filepath.Join("testdata", "workloads", jobParamsPath)
		wantedCFNParamsBytes, err := ioutil.ReadFile(path)
		require.NoError(t, err)

		require.Equal(t, string(wantedCFNParamsBytes), actualParams)
	})

}
