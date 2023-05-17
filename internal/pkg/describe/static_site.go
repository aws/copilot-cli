// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"

	"github.com/dustin/go-humanize/english"
)

const (
	staticSiteOutputCFDomainName    = "CloudFrontDistributionDomainName"
	staticSiteOutputCFAltDomainName = "CloudFrontDistributionAlternativeDomainName"
)

// StaticSiteDescriber retrieves information about a static site service.
type StaticSiteDescriber struct {
	app string
	svc string

	initWkldStackDescriber func(string) (workloadDescriber, error)
	wkldDescribers         map[string]workloadDescriber
}

// NewStaticSiteDescriber instantiates a static site service describer.
func NewStaticSiteDescriber(opt NewServiceConfig) (*StaticSiteDescriber, error) {
	describer := &StaticSiteDescriber{
		app:            opt.App,
		svc:            opt.Svc,
		wkldDescribers: make(map[string]workloadDescriber),
	}
	describer.initWkldStackDescriber = func(env string) (workloadDescriber, error) {
		if describer, ok := describer.wkldDescribers[env]; ok {
			return describer, nil
		}
		svcDescr, err := newWorkloadStackDescriber(workloadConfig{
			app:         opt.App,
			name:        opt.Svc,
			configStore: opt.ConfigStore,
		}, env)
		if err != nil {
			return nil, err
		}
		describer.wkldDescribers[env] = svcDescr
		return svcDescr, nil
	}
	return describer, nil
}

// URI returns the public accessible URI of a static site service.
func (d *StaticSiteDescriber) URI(envName string) (URI, error) {
	wkldDescr, err := d.initWkldStackDescriber(envName)
	if err != nil {
		return URI{}, err
	}
	outputs, err := wkldDescr.Outputs()
	if err != nil {
		return URI{}, fmt.Errorf("get stack output for service %q: %w", d.svc, err)
	}
	uris := []string{outputs[staticSiteOutputCFDomainName]}
	if outputs[staticSiteOutputCFAltDomainName] != "" {
		uris = append(uris, outputs[staticSiteOutputCFAltDomainName])
	}
	return URI{
		URI:        english.OxfordWordSeries(uris, "or"),
		AccessType: URIAccessTypeInternet,
	}, nil
}
