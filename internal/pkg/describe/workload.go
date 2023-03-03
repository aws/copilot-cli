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
	"gopkg.in/yaml.v3"
)

// workloadStackDescriber provides base functionality for retrieving info about a workload stack.
type workloadStackDescriber struct {
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

type workloadConfig struct {
	app         string
	name        string
	configStore ConfigStoreSvc
}

// newWorkloadStackDescriber instantiates the core elements of a new workload.
func newWorkloadStackDescriber(opt workloadConfig, env string) (*workloadStackDescriber, error) {
	environment, err := opt.configStore.GetEnvironment(opt.app, env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", env, err)
	}
	sess, err := sessions.ImmutableProvider().FromRole(environment.ManagerRoleARN, environment.Region)
	if err != nil {
		return nil, err
	}
	return &workloadStackDescriber{
		app:  opt.app,
		name: opt.name,
		env:  env,

		cfn:  stack.NewStackDescriber(cfnstack.NameForWorkload(opt.app, env, opt.name), sess),
		sess: sess,
	}, nil
}

// Params returns the parameters of the workload stack.
func (d *workloadStackDescriber) Params() (map[string]string, error) {
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
func (d *workloadStackDescriber) Outputs() (map[string]string, error) {
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
func (d *workloadStackDescriber) StackResources() ([]*stack.Resource, error) {
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
func (d *workloadStackDescriber) Manifest() ([]byte, error) {
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
