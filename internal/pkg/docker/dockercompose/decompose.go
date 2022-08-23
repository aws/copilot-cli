// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"fmt"
	"github.com/compose-spec/compose-go/loader"
	compose "github.com/compose-spec/compose-go/types"
	"github.com/mitchellh/mapstructure"
	"sort"
)

type composeServices map[string]map[string]any

// DecomposeService parses a Compose YAML file and then converts a single service to a Copilot manifest.
func DecomposeService(content []byte, svcName string, workingDir string) (*ConvertedService, IgnoredKeys, error) {
	config, err := loader.ParseYAML(content)
	if err != nil {
		return nil, nil, fmt.Errorf("parse compose yaml: %w", err)
	}

	services, err := getServices(config)
	if err != nil {
		return nil, nil, err
	}

	service, err := serviceConfig(services, svcName)
	if err != nil {
		return nil, nil, err
	}

	ignored, err := unsupportedServiceKeys(service, svcName)
	if err != nil {
		return nil, nil, err
	}

	// workaround for compose-go automatically loading env files from disk and merging their
	// content into the "environment" field (the equivalent of the Copilot variables key)
	// TODO (rclinard-amzn): make a PR to compose-go instead of using this workaround
	envFiles, err := isolateEnvFiles(service)
	if err != nil {
		return nil, nil, fmt.Errorf("isolate env file(s): %w", err)
	}
	removeEnvFiles(services)

	project, err := loader.Load(compose.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []compose.ConfigFile{
			{
				Config: config,
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("load Compose project: %w", err)
	}

	svcConfig, err := project.GetService(svcName)
	if err != nil {
		panic("impossible: project.GetService failed even though we have already checked that the " +
			"service is valid and exists")
	}

	svcConfig.EnvFile = envFiles
	svc, svcIgnored, err := convertService(&svcConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("convert Compose service to Copilot manifest: %w", err)
	}

	ignored = append(ignored, svcIgnored...)
	sort.Strings(ignored)

	return svc, ignored, nil
}

func getServices(config map[string]any) (composeServices, error) {
	svcs, ok := config["services"]
	if !ok || svcs == nil {
		return nil, fmt.Errorf("compose file has no services")
	}

	services, ok := svcs.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("\"services\" top-level element was not a map, was: %v", svcs)
	}

	typedServices := make(composeServices)

	for name, svc := range services {
		service, ok := svc.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("\"services.%s\" element was not a map", name)
		}
		typedServices[name] = service
	}

	return typedServices, nil
}

// serviceConfig extracts the map corresponding to a Compose service from the parsed Compose config map.
func serviceConfig(services composeServices, svcName string) (map[string]any, error) {
	service, ok := services[svcName]
	if ok {
		return service, nil
	}

	var validNames []string
	for svc := range services {
		validNames = append(validNames, svc)
	}
	sort.Strings(validNames)
	return nil, fmt.Errorf("no service named \"%s\" in this Compose file, valid services are: %v", svcName, validNames)
}

// unsupportedServiceKeys scans over fields in the parsed yaml to find unsupported keys.
func unsupportedServiceKeys(service map[string]any, svcName string) (IgnoredKeys, error) {
	var ignored, fatal []string

	for key := range service {
		if ignoredServiceKeys[key] {
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

// isolateEnvFiles manually handles the env_file key so that compose-go does not try to read the env files from the disk.
func isolateEnvFiles(service map[string]interface{}) ([]string, error) {
	envFileStr := struct {
		EnvFile string `mapstructure:"env_file"`
	}{}
	err := mapstructure.Decode(service, &envFileStr)

	if err == nil && envFileStr.EnvFile != "" {
		return []string{envFileStr.EnvFile}, nil
	}

	envFileOnly := struct {
		EnvFile []string `mapstructure:"env_file"`
	}{}
	err = mapstructure.Decode(service, &envFileOnly)

	if err == nil {
		return envFileOnly.EnvFile, nil
	}

	return nil, err
}

func removeEnvFiles(services map[string]map[string]interface{}) {
	for _, svc := range services {
		delete(svc, "env_file")
	}
}
