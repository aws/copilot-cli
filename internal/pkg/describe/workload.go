// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"gopkg.in/yaml.v3"
)

// WorkloadStackDescriber provides base functionality for retrieving info about a workload stack.
type WorkloadStackDescriber struct {
	app  string
	name string
	env  string

	cfn  stackDescriber
	sess *session.Session

	// Cache variables.
	params         map[string]string
	outputs        map[string]string
	stackResources []*stack.Resource
}

// NewWorkloadConfig contains fields that initiates workload describer struct.
type NewWorkloadConfig struct {
	App         string
	Env         string
	Name        string
	ConfigStore ConfigStoreSvc
}

// NewWorkloadStackDescriber instantiates the core elements of a new workload.
func NewWorkloadStackDescriber(opt NewWorkloadConfig) (*WorkloadStackDescriber, error) {
	environment, err := opt.ConfigStore.GetEnvironment(opt.App, opt.Env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", opt.Env, err)
	}
	sess, err := sessions.ImmutableProvider().FromRole(environment.ManagerRoleARN, environment.Region)
	if err != nil {
		return nil, err
	}
	return &WorkloadStackDescriber{
		app:  opt.App,
		name: opt.Name,
		env:  opt.Env,

		cfn:  stack.NewStackDescriber(cfnstack.NameForWorkload(opt.App, opt.Env, opt.Name), sess),
		sess: sess,
	}, nil
}

// Version returns the CloudFormation template version associated with
// the workload by reading the Metadata.Version field from the template.
//
// If the Version field does not exist, then it's a legacy template and it returns an version.LegacyWorkloadTemplate and nil error.
func (d *WorkloadStackDescriber) Version() (string, error) {
	return stackVersion(d.cfn, version.LegacyWorkloadTemplate)
}

func stackVersion(descr stackDescriber, legacyVersion string) (string, error) {
	raw, err := descr.StackMetadata()
	if err != nil {
		return "", err
	}
	metadata := struct {
		Version string `yaml:"Version"`
	}{}
	if err := yaml.Unmarshal([]byte(raw), &metadata); err != nil {
		return "", fmt.Errorf("unmarshal Metadata property to read Version: %w", err)
	}
	if metadata.Version == "" {
		return legacyVersion, nil
	}
	return metadata.Version, nil
}

// Params returns the parameters of the workload stack.
func (d *WorkloadStackDescriber) Params() (map[string]string, error) {
	if d.params != nil {
		return d.params, nil
	}
	descr, err := d.cfn.Describe()
	if err != nil {
		return nil, err
	}
	d.params = descr.Parameters
	return descr.Parameters, nil
}

// Outputs returns the outputs of the service stack.
func (d *WorkloadStackDescriber) Outputs() (map[string]string, error) {
	if d.outputs != nil {
		return d.outputs, nil
	}
	descr, err := d.cfn.Describe()
	if err != nil {
		return nil, err
	}
	d.outputs = descr.Outputs
	return descr.Outputs, nil
}

// StackResources returns the workload stack resources created by CloudFormation.
func (d *WorkloadStackDescriber) StackResources() ([]*stack.Resource, error) {
	if len(d.stackResources) != 0 {
		return d.stackResources, nil
	}
	svcResources, err := d.cfn.Resources()
	if err != nil {
		return nil, err
	}
	var resources []*stack.Resource
	ignored := struct{}{}
	ignoredResources := map[string]struct{}{
		rulePriorityFunction: ignored,
		waitCondition:        ignored,
		waitConditionHandle:  ignored,
	}
	for _, svcResource := range svcResources {
		if _, ok := ignoredResources[svcResource.Type]; !ok {
			resources = append(resources, svcResource)
		}
	}
	d.stackResources = resources
	return resources, nil
}

// Manifest returns the contents of the manifest used to deploy a workload stack.
// If the Manifest metadata doesn't exist in the stack template, then returns ErrManifestNotFoundInTemplate.
func (d *WorkloadStackDescriber) Manifest() ([]byte, error) {
	tpl, err := d.cfn.StackMetadata()
	if err != nil {
		return nil, fmt.Errorf("retrieve stack metadata for %s-%s-%s: %w", d.app, d.env, d.name, err)
	}

	metadata := struct {
		Manifest string `yaml:"Manifest"`
	}{}
	if err := yaml.Unmarshal([]byte(tpl), &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal Metadata.Manifest in stack %s-%s-%s: %v", d.app, d.env, d.name, err)
	}
	if len(strings.TrimSpace(metadata.Manifest)) == 0 {
		return nil, &ErrManifestNotFoundInTemplate{
			app:  d.app,
			env:  d.env,
			name: d.name,
		}
	}
	return []byte(metadata.Manifest), nil
}
