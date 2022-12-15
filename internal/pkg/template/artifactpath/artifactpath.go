// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package artifactpath holds functions to generate the S3 object path for artifacts.
package artifactpath

import (
	"crypto/sha256"
	"fmt"
	"path"
	"path/filepath"
)

const (
	s3ArtifactDirName           = "manual"
	s3TemplateDirName           = "templates"
	s3ArtifactAddonsDirName     = "addons"
	s3ArtifactAddonAssetDirName = "assets"
	s3ArtifactEnvFilesDirName   = "env-files"
	s3ScriptsDirName            = "scripts"
	s3CustomResourcesDirName    = "custom-resources"
	s3EnvironmentsAddonsDirName = "environments"
)

// MkdirSHA256 prefixes the key with the SHA256 hash of the contents of "manual/<hash>/key".
func MkdirSHA256(key string, content []byte) string {
	return path.Join(s3ArtifactDirName, fmt.Sprintf("%x", sha256.Sum256(content)), key)
}

// Addons returns the path to store addon artifact files with sha256 of the content.
// Example: manual/addons/key/sha.yml.
func Addons(key string, content []byte) string {
	return path.Join(s3ArtifactDirName, s3ArtifactAddonsDirName, key, fmt.Sprintf("%x.yml", sha256.Sum256(content)))
}

// AddonAsset returns the path to store an addon asset file.
// Example: manual/addons/frontend/assets/668e2b73ac.
func AddonAsset(workloadName, hash string) string {
	return path.Join(s3ArtifactDirName, s3ArtifactAddonsDirName, workloadName, s3ArtifactAddonAssetDirName, hash)
}

// EnvironmentAddons returns the path to store environment addon artifact files with sha256 of the content.
// Example: manual/addons/environments/sha.yml.
func EnvironmentAddons(content []byte) string {
	return path.Join(s3ArtifactDirName, s3ArtifactAddonsDirName, s3EnvironmentsAddonsDirName, fmt.Sprintf("%x.yml", sha256.Sum256(content)))
}

// EnvironmentAddonAsset returns the path to store an addon asset file for an environment addon.
// Example: manual/addons/environments/assets/668e2b73ac.
func EnvironmentAddonAsset(hash string) string {
	return path.Join(s3ArtifactDirName, s3ArtifactAddonsDirName, s3EnvironmentsAddonsDirName, s3ArtifactAddonAssetDirName, hash)
}

// CFNTemplate returns the path to store cloudformation templates with sha256 of the content.
// Example: manual/templates/key/sha.yml.
func CFNTemplate(key string, content []byte) string {
	return path.Join(s3ArtifactDirName, s3TemplateDirName, key, fmt.Sprintf("%x.yml", sha256.Sum256(content)))
}

// EnvFiles returns the path to store an env file artifact with sha256 of the content..
// Example: manual/env-files/key/sha.env.
func EnvFiles(key string, content []byte) string {
	// use filepath.Base to prevent cryptic errors in the ecs agent for paths like "..\magic.env"
	return path.Join(s3ArtifactDirName, s3ArtifactEnvFilesDirName, filepath.Base(key), fmt.Sprintf("%x.env", sha256.Sum256(content)))
}

// CustomResource returns the path to store a custom resource with a sha256 of the contents of the file.
// Example: manual/scripts/custom-resources/key/sha.zip
func CustomResource(key string, zipFile []byte) string {
	return path.Join(s3ArtifactDirName, s3ScriptsDirName, s3CustomResourcesDirName, key, fmt.Sprintf("%x.zip", sha256.Sum256(zipFile)))
}
