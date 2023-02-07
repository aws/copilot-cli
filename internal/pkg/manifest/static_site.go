// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/imdario/mergo"
)

const (
	staticSiteManifestPath = "workloads/services/static-site/manifest.yml"
)

// StaticSite holds the configuration to configure and upload static assets to the static site service.
type StaticSite struct {
	Workload         `yaml:",inline"`
	StaticSiteConfig `yaml:",inline"`
	// Use *StaticSiteConfig because of https://github.com/imdario/mergo/issues/146
	Environments map[string]*StaticSiteConfig `yaml:",flow"` // Fields to override per environment.

	parser template.Parser
}

// StaticSiteConfig holds the configuration for a static site service.
type StaticSiteConfig struct {
	FileUploads []FileUpload `yaml:"files"`
}

// FileUpload represents the options for file uploading.
type FileUpload struct {
	Source      string              `yaml:"source"`
	Destination string              `yaml:"destination"`
	Context     string              `yaml:"context"`
	Recursive   bool                `yaml:"recursive"`
	Reinclude   StringSliceOrString `yaml:"reinclude"`
	Exclude     StringSliceOrString `yaml:"exclude"`
}

// NewStaticSite creates a new static site service.
func NewStaticSite(name string) *StaticSite {
	return &StaticSite{
		Workload: Workload{
			Name: aws.String(name),
			Type: aws.String(StaticSiteType),
		},
		parser: template.New(),
	}
}

func (s StaticSite) applyEnv(envName string) (workloadManifest, error) {
	overrideConfig, ok := s.Environments[envName]
	if !ok {
		return &s, nil
	}

	if overrideConfig == nil {
		return &s, nil
	}

	// Apply overrides to the original service s.
	for _, t := range defaultTransformers {
		err := mergo.Merge(&s, StaticSite{
			StaticSiteConfig: *overrideConfig,
		}, mergo.WithOverride, mergo.WithTransformers(t))
		if err != nil {
			return nil, err
		}
	}
	s.Environments = nil
	return &s, nil
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (s *StaticSite) MarshalBinary() ([]byte, error) {
	content, err := s.parser.Parse(staticSiteManifestPath, *s)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// To implement workloadManifest.
func (s *StaticSite) subnets() *SubnetListOrArgs {
	return nil
}

// To implement workloadManifest.
func (s *StaticSite) requiredEnvironmentFeatures() []string {
	return nil
}
