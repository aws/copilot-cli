// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/compose-spec/compose-go/loader"
	compose "github.com/compose-spec/compose-go/types"
	"sort"
)

// DecomposeService parses a Compose YAML file and then converts a single service to a Copilot manifest.
func DecomposeService(content []byte, svcName string, workingDir string) (*manifest.BackendServiceConfig, IgnoredKeys, error) {
	config, err := loader.ParseYAML(content)
	if err != nil {
		return nil, nil, fmt.Errorf("parse compose yaml: %w", err)
	}

	service, err := serviceConfig(config, svcName)
	if err != nil {
		return nil, nil, fmt.Errorf("get service config: %w", err)
	}

	ignored, err := unsupportedServiceKeys(service, svcName)
	if err != nil {
		return nil, nil, fmt.Errorf("get unsupported service keys: %w", err)
	}

	project, err := loader.Load(compose.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []compose.ConfigFile{
			{
				Content: content,
				Config:  config,
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("load Compose project: %w", err)
	}

	svcConfig, err := project.GetService(svcName)
	if err != nil {
		// I don't think this is possible due to the previous check,
		// but I'm not confident enough in that assessment to make this a panic.
		return nil, nil, fmt.Errorf("get service from Compose project: %w", err)
	}

	// TODO: Port handling & exposed port detection, to be implemented in Milestone 3
	var port uint16 = 80
	backendSvc, svcIgnored, err := convertBackendService(&svcConfig, port)
	if err != nil {
		return nil, nil, fmt.Errorf("convert Compose service to Copilot manifest: %w", err)
	}
	ignored = append(ignored, svcIgnored...)

	sort.Strings(ignored)
	return backendSvc, ignored, nil
}

// serviceConfig extracts the map corresponding to a Compose service from the parsed Compose config map.
func serviceConfig(config map[string]interface{}, svcName string) (map[string]interface{}, error) {
	svcs, ok := config["services"]
	if !ok || svcs == nil {
		return nil, fmt.Errorf("compose file has no services")
	}

	services, ok := svcs.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("\"services\" top-level element was not a map, was: %v", svcs)
	}

	svc, ok := services[svcName]
	if !ok {
		var validNames []string
		for svc := range services {
			validNames = append(validNames, svc)
		}
		sort.Strings(validNames)
		return nil, fmt.Errorf("no service named \"%s\" in this Compose file, valid services are: %v", svcName, validNames)
	}

	service, ok := svc.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("\"services.%s\" element was not a map", svcName)
	}

	return service, nil
}

// unsupportedServiceKeys scans over fields in the parsed yaml to find unsupported keys.
func unsupportedServiceKeys(service map[string]interface{}, svcName string) (IgnoredKeys, error) {
	var ignored, fatal []string

	for key := range service {
		if _, ok := ignoredServiceKeys[key]; ok {
			ignored = append(ignored, key)
		} else if _, ok := fatalServiceKeys[key]; ok {
			fatal = append(fatal, key)
		}
	}

	if len(fatal) != 0 {
		// sort so we have consistent (testable) error messages
		sort.Strings(fatal)
		return nil, fmt.Errorf("\"services.%s\" relies on fatally-unsupported Compose keys: %v", svcName, fatal)
	}

	return ignored, nil
}
