// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"net/url"

	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
)

const (
	svcOutputURL = "ServiceURL"
)

type resourceGetter interface {
	GetResourcesByTags(resourceTypes []string, tags map[string]string) ([]*rg.Resource, error)
}

// RDWebServiceDescriber retrieves information about a load balanced web service.
type RDWebServiceDescriber struct {
	app             string
	svc             string
	enableResources bool

	store                DeployedEnvServicesLister
	svcDescriber         map[string]appRunnerSvcDescriber
	initServiceDescriber func(string) error
}

// NewRDWebServiceConfig contains fields that initiates RDWebServiceDescriber struct.
type NewRDWebServiceConfig struct {
	NewServiceConfig
	EnableResources bool
	DeployStore     DeployedEnvServicesLister
}

// NewRDWebServiceDescriber instantiates a load balanced service describer.
func NewRDWebServiceDescriber(opt NewRDWebServiceConfig) (*RDWebServiceDescriber, error) {
	describer := &RDWebServiceDescriber{
		app:             opt.App,
		svc:             opt.Svc,
		enableResources: opt.EnableResources,
		store:           opt.DeployStore,
		svcDescriber:    make(map[string]appRunnerSvcDescriber),
	}
	describer.initServiceDescriber = func(env string) error {
		if _, ok := describer.svcDescriber[env]; ok {
			return nil
		}
		d, err := NewServiceDescriber(NewServiceConfig{
			App:         opt.App,
			Env:         env,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return err
		}
		describer.svcDescriber[env] = d
		return nil
	}
	return describer, nil
}

// URI returns the WebServiceURI to identify this service uniquely given an environment name.
func (d *RDWebServiceDescriber) URI(envName string) (string, error) {
	err := d.initServiceDescriber(envName)
	if err != nil {
		return "", err
	}

	svcOutputs, err := d.svcDescriber[envName].SvcOutputs()
	if err != nil {
		return "", fmt.Errorf("get outputs for service %s: %w", d.svc, err)
	}

	svcUrl := &url.URL{
		Host: svcOutputs[svcOutputURL],
		// App Runner defaults to https
		Scheme: "https",
	}

	return svcUrl.String(), nil
}
