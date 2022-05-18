// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

type deployEnvVars struct {
	appName      string
	name         string
	isProduction bool
}

type deployEnvOpts struct {
	deployEnvVars

	// Dependencies.
	store *config.Store

	// Dependencies to execute.
	ws           wsEnvironmentReader
	deployer     envDeployer
	identity     identityService
	interpolator interpolator

	// Cached variables.
	targetApp *config.Application
	targetEnv *config.Environment

	// Functions to facilitate testing.
	unmarshalManifest func(in []byte) (*manifest.Environment, error)
}

// Execute deploys an environment given a manifest.
func (o *deployEnvOpts) Execute() error {
	mft, err := o.environmentManifest()
	if err != nil {
		return err
	}
	caller, err := o.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	urls, err := o.deployer.UploadArtifacts()
	if err != nil {
		return fmt.Errorf("upload artifacts for environment %s: %w", o.name, err)
	}
	if err := o.deployer.DeployEnvironment(&deploy.DeployEnvironmentInput{
		RootUserARN:         caller.RootUserARN,
		IsProduction:        o.isProduction,
		CustomResourcesURLs: urls,
		Manifest:            mft,
	}); err != nil {
		return fmt.Errorf("deploy environment %s: %w", o.name, err)
	}
	return nil
}

func (o *deployEnvOpts) environmentManifest() (*manifest.Environment, error) {
	targetEnv, err := o.cachedTargetEnv()
	if err != nil {
		return nil, err
	}
	raw, err := o.ws.ReadEnvironmentManifest(targetEnv.Name)
	if err != nil {
		return nil, fmt.Errorf("read manifest for environment %s: %w", targetEnv.Name, err)
	}
	interpolated, err := o.interpolator.Interpolate(string(raw))
	if err != nil {
		return nil, fmt.Errorf("interpolate environment variables for %s manifest: %w", targetEnv.Name, err)
	}
	mft, err := o.unmarshalManifest([]byte(interpolated))
	if err != nil {
		return nil, fmt.Errorf("unmarshal environment manifest for %s: %w", targetEnv.Name, err)
	}
	if err := mft.Validate(); err != nil {
		return nil, fmt.Errorf("validate environment manifest for %s: %w", targetEnv.Name, err)
	}
	return mft, nil
}

func (o *deployEnvOpts) cachedTargetEnv() (*config.Environment, error) {
	if o.targetEnv == nil {
		env, err := o.store.GetEnvironment(o.appName, o.name)
		if err != nil {
			return nil, err
		}
		o.targetEnv = env
	}
	return o.targetEnv, nil
}
