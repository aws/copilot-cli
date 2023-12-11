// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

// StaticSite represents the configuration needed to create a CloudFormation stack from a static site service manifest.
type StaticSite struct {
	*wkld
	manifest             *manifest.StaticSite
	dnsDelegationEnabled bool
	appInfo              deploy.AppInformation

	parser          staticSiteReadParser
	assetMappingURL string
}

// StaticSiteConfig contains fields to configure StaticSite.
type StaticSiteConfig struct {
	App                *config.Application
	EnvManifest        *manifest.Environment
	Manifest           *manifest.StaticSite
	RawManifest        string
	RuntimeConfig      RuntimeConfig
	RootUserARN        string
	ArtifactBucketName string
	ArtifactKey        string
	Addons             NestedStackConfigurer
	AssetMappingURL    string
}

// NewStaticSite creates a new CFN stack from a manifest file, given the options.
func NewStaticSite(cfg *StaticSiteConfig) (*StaticSite, error) {
	crs, err := customresource.StaticSite(fs)
	if err != nil {
		return nil, fmt.Errorf("static site custom resources: %w", err)
	}
	cfg.RuntimeConfig.loadCustomResourceURLs(cfg.ArtifactBucketName, uploadableCRs(crs).convert())

	var dnsDelegationEnabled bool
	var appInfo deploy.AppInformation
	if cfg.App.Domain != "" {
		dnsDelegationEnabled = true
		appInfo = deploy.AppInformation{
			Name:                cfg.App.Name,
			Domain:              cfg.App.Domain,
			AccountPrincipalARN: cfg.RootUserARN,
		}
	}
	return &StaticSite{
		wkld: &wkld{
			name:               aws.StringValue(cfg.Manifest.Name),
			env:                aws.StringValue(cfg.EnvManifest.Name),
			app:                cfg.App.Name,
			permBound:          cfg.App.PermissionsBoundary,
			artifactBucketName: cfg.ArtifactBucketName,
			artifactKey:        cfg.ArtifactKey,
			rc:                 cfg.RuntimeConfig,
			rawManifest:        cfg.RawManifest,
			parser:             fs,
			addons:             cfg.Addons,
		},
		manifest:             cfg.Manifest,
		dnsDelegationEnabled: dnsDelegationEnabled,
		appInfo:              appInfo,

		parser:          fs,
		assetMappingURL: cfg.AssetMappingURL,
	}, nil
}

// Template returns the CloudFormation template for the service parametrized for the environment.
func (s *StaticSite) Template() (string, error) {
	crs, err := convertCustomResources(s.rc.CustomResourcesURL)
	if err != nil {
		return "", err
	}
	addonsParams, err := s.addonsParameters()
	if err != nil {
		return "", err
	}
	addonsOutputs, err := s.addonsOutputs()
	if err != nil {
		return "", err
	}
	var bucket, path string
	if s.assetMappingURL != "" {
		bucket, path, err = s3.ParseURL(s.assetMappingURL)
		if err != nil {
			return "", err
		}
	}

	var staticSiteAlias string
	if s.appInfo.Domain != "" {
		// NLB, App Runner, and LBWS use AWS::Route53::RecordSetGroup to
		// create the A-Record to route traffic to the LoadBalancer endpoint.
		// Static Site default domain alias is created by a custom resource.
		staticSiteAlias = fmt.Sprintf("%s.%s.%s.%s", s.name, s.env, s.app, s.appInfo.Domain)
	}
	if s.manifest.HTTP.Alias != "" {
		staticSiteAlias = s.manifest.HTTP.Alias
	}
	dnsDelegationRole, dnsName := convertAppInformation(s.appInfo)
	content, err := s.parser.ParseStaticSite(template.WorkloadOpts{
		// Workload parameters.
		AppName:            s.app,
		EnvName:            s.env,
		EnvVersion:         s.rc.EnvVersion,
		Version:            s.rc.Version,
		SerializedManifest: string(s.rawManifest),
		WorkloadName:       s.name,
		WorkloadType:       manifestinfo.StaticSiteType,

		// Additional options that are common between **all** workload templates.
		AddonsExtraParams:   addonsParams,
		NestedStack:         addonsOutputs,
		PermissionsBoundary: s.permBound,

		// Custom Resource Config.
		CustomResources: crs,

		AppDNSName:             dnsName,
		AppDNSDelegationRole:   dnsDelegationRole,
		AssetMappingFileBucket: bucket,
		AssetMappingFilePath:   path,
		StaticSiteAlias:        staticSiteAlias,
		StaticSiteCert:         s.manifest.HTTP.Certificate,
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (s *StaticSite) Parameters() ([]*cloudformation.Parameter, error) {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadAppNameParamKey),
			ParameterValue: aws.String(s.app),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvNameParamKey),
			ParameterValue: aws.String(s.env),
		},
		{
			ParameterKey:   aws.String(WorkloadNameParamKey),
			ParameterValue: aws.String(s.name),
		},
		{
			ParameterKey:   aws.String(WorkloadAddonsTemplateURLParamKey),
			ParameterValue: aws.String(s.rc.AddonsTemplateURL),
		},
		{
			ParameterKey:   aws.String(WorkloadArtifactKeyARNParamKey),
			ParameterValue: aws.String(s.wkld.artifactKey),
		},
	}, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized to a JSON document.
func (s *StaticSite) SerializedParameters() (string, error) {
	return serializeTemplateConfig(s.wkld.parser, s)
}
