// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

// StaticSite represents the configuration needed to create a CloudFormation stack from a static site service manifest.
type StaticSite struct {
	*wkld
	manifest *manifest.StaticSite
	appInfo  deploy.AppInformation

	parser   staticSiteReadParser
	localCRs []uploadable // Custom resources that have not been uploaded yet.
}

// StaticSiteConfig contains fields to configure StaticSite.
type StaticSiteConfig struct {
	App                *config.Application
	EnvManifest        *manifest.Environment
	Manifest           *manifest.StaticSite
	RawManifest        []byte // Content of the manifest file without any transformations.
	RuntimeConfig      RuntimeConfig
	RootUserARN        string
	ArtifactBucketName string
	Addons             NestedStackConfigurer
}

// NewStaticSite creates a new CFN stack from a manifest file, given the options.
func NewStaticSite(conf StaticSiteConfig) (*StaticSite, error) {
	crs, err := customresource.StaticSite(fs)
	if err != nil {
		return nil, fmt.Errorf("static site custom resources: %w", err)
	}
	return &StaticSite{
		wkld: &wkld{
			name:               aws.StringValue(conf.Manifest.Name),
			env:                aws.StringValue(conf.EnvManifest.Name),
			app:                conf.App.Name,
			permBound:          conf.App.PermissionsBoundary,
			artifactBucketName: conf.ArtifactBucketName,
			rc:                 conf.RuntimeConfig,
			rawManifest:        conf.RawManifest,
			parser:             fs,
			addons:             conf.Addons,
		},
		manifest: conf.Manifest,

		parser:   fs,
		localCRs: uploadableCRs(crs).convert(),
	}, nil
}

// Template returns the CloudFormation template for the service parametrized for the environment.
func (s *StaticSite) Template() (string, error) {
	return "", nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *StaticSite) Parameters() ([]*cloudformation.Parameter, error) {
	return nil, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized to a JSON document.
func (s *StaticSite) SerializedParameters() (string, error) {
	return serializeTemplateConfig(s.wkld.parser, s)
}
