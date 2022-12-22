// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package artifactpath

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomResource(t *testing.T) {
	require.Equal(t, "manual/scripts/custom-resources/envcontrollerfunction/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.zip", CustomResource("envcontrollerfunction", []byte("")))
}

func TestEnvironmentAddons(t *testing.T) {
	require.Equal(t, "manual/addons/environments/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.yml", EnvironmentAddons([]byte("")))
}

func TestEnvironmentAddonsAsset(t *testing.T) {
	require.Equal(t, "manual/addons/environments/assets/hash", EnvironmentAddonAsset("hash"))
}
