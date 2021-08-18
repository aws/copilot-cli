// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// WorkerServiceDescriber retrieves information about a worker service.
type WorkerServiceDescriber struct {
	*ecsServiceDescriber
}

// NewWorkerServiceDescriber instantiates a worker service describer.
func NewWorkerServiceDescriber(opt NewServiceConfig) (*WorkerServiceDescriber, error) {
	describer := &WorkerServiceDescriber{
		ecsServiceDescriber: &ecsServiceDescriber{
			app:               opt.App,
			svc:               opt.Svc,
			enableResources:   opt.EnableResources,
			store:             opt.DeployStore,
			svcStackDescriber: make(map[string]ecsStackDescriber),
		},
	}
	describer.initDescribers = func(env string) error {
		if _, ok := describer.svcStackDescriber[env]; ok {
			return nil
		}
		d, err := NewECSServiceDescriber(NewServiceConfig{
			App:         opt.App,
			Env:         env,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return err
		}
		describer.svcStackDescriber[env] = d
		return nil
	}
	return describer, nil
}

// URI returns the service discovery namespace and is used to make
// WorkerServiceDescriber have the same signature as WebServiceDescriber.
func (d *WorkerServiceDescriber) URI(envName string) (string, error) {
	return BlankServiceDiscoveryURI, nil
}

// Describe returns info of a worker service.
func (d *WorkerServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironmentsDeployedTo(d.app, d.svc)
	if err != nil {
		return nil, fmt.Errorf("list deployed environments for application %s: %w", d.app, err)
	}

	var configs []*ECSServiceConfig
	var envVars []*containerEnvVar
	var secrets []*secret
	for _, env := range environments {
		err := d.initDescribers(env)
		if err != nil {
			return nil, err
		}
		svcParams, err := d.svcStackDescriber[env].Params()
		if err != nil {
			return nil, fmt.Errorf("get stack parameters for environment %s: %w", env, err)
		}
		configs = append(configs, &ECSServiceConfig{
			ServiceConfig: &ServiceConfig{
				Environment: env,
				Port:        blankContainerPort,
				CPU:         svcParams[cfnstack.WorkloadTaskCPUParamKey],
				Memory:      svcParams[cfnstack.WorkloadTaskMemoryParamKey],
			},
			Tasks: svcParams[cfnstack.WorkloadTaskCountParamKey],
		})
		workerSvcEnvVars, err := d.svcStackDescriber[env].EnvVars()
		if err != nil {
			return nil, fmt.Errorf("retrieve environment variables: %w", err)
		}
		envVars = append(envVars, flattenContainerEnvVars(env, workerSvcEnvVars)...)
		webSvcSecrets, err := d.svcStackDescriber[env].Secrets()
		if err != nil {
			return nil, fmt.Errorf("retrieve secrets: %w", err)
		}
		secrets = append(secrets, flattenSecrets(env, webSvcSecrets)...)
	}

	resources := make(map[string][]*stack.Resource)
	if d.enableResources {
		for _, env := range environments {
			err := d.initDescribers(env)
			if err != nil {
				return nil, err
			}
			stackResources, err := d.svcStackDescriber[env].ServiceStackResources()
			if err != nil {
				return nil, fmt.Errorf("retrieve service resources: %w", err)
			}
			resources[env] = stackResources
		}
	}

	return &workerSvcDesc{
		Service:        d.svc,
		Type:           manifest.WorkerServiceType,
		App:            d.app,
		Configurations: configs,
		Variables:      envVars,
		Secrets:        secrets,
		Resources:      resources,

		environments: environments,
	}, nil
}

// workerSvcDesc contains serialized parameters for a worker service.
type workerSvcDesc struct {
	Service        string               `json:"service"`
	Type           string               `json:"type"`
	App            string               `json:"application"`
	Configurations ecsConfigurations    `json:"configurations"`
	Variables      containerEnvVars     `json:"variables"`
	Secrets        secrets              `json:"secrets,omitempty"`
	Resources      deployedSvcResources `json:"resources,omitempty"`

	environments []string `json:"-"`
}

// JSONString returns the stringified workerService struct with json format.
func (w *workerSvcDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal worker service description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified workerService struct with human readable format.
func (w *workerSvcDesc) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Application", w.App)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", w.Service)
	fmt.Fprintf(writer, "  %s\t%s\n", "Type", w.Type)
	fmt.Fprint(writer, color.Bold.Sprint("\nConfigurations\n\n"))
	writer.Flush()
	w.Configurations.humanString(writer)
	fmt.Fprint(writer, color.Bold.Sprint("\nVariables\n\n"))
	writer.Flush()
	w.Variables.humanString(writer)
	if len(w.Secrets) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nSecrets\n\n"))
		writer.Flush()
		w.Secrets.humanString(writer)
	}
	if len(w.Resources) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nResources\n"))
		writer.Flush()

		w.Resources.humanStringByEnv(writer, w.environments)
	}
	writer.Flush()
	return b.String()
}
