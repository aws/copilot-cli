// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
)

type subnetIDsGetter interface {
	SubnetIDs(filters ...ec2.Filter) ([]string, error)
}

type loader interface {
	load() error
}

// DynamicWorkloadManifest represents a dynamically populated workload manifest.
type DynamicWorkloadManifest struct {
	mft workloadManifest

	// Clients required to dynamically populate.
	newSubnetIDsGetter func(*session.Session) subnetIDsGetter
}

func newDynamicWorkloadManifest(mft workloadManifest) *DynamicWorkloadManifest {
	return &DynamicWorkloadManifest{
		mft: mft,
		newSubnetIDsGetter: func(s *session.Session) subnetIDsGetter {
			return ec2.New(s)
		},
	}
}

// Manifest returns the manifest content.
func (s *DynamicWorkloadManifest) Manifest() any {
	return s.mft
}

// ApplyEnv returns the workload manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (s DynamicWorkloadManifest) ApplyEnv(envName string) (DynamicWorkload, error) {
	mft, err := s.mft.applyEnv(envName)
	if err != nil {
		return nil, err
	}
	s.mft = mft
	return &s, nil
}

// RequiredEnvironmentFeatures returns environment features that are required for this manifest.
func (s *DynamicWorkloadManifest) RequiredEnvironmentFeatures() []string {
	return s.mft.requiredEnvironmentFeatures()
}

// Load dynamically populates all fields in the manifest.
func (s *DynamicWorkloadManifest) Load(sess *session.Session) error {
	loaders := []loader{
		&dynamicSubnets{
			cfg:    s.mft.subnets(),
			client: s.newSubnetIDsGetter(sess),
		},
	}
	return loadAll(loaders)
}

func loadAll(loaders []loader) error {
	for _, loader := range loaders {
		if err := loader.load(); err != nil {
			return err
		}
	}
	return nil
}
