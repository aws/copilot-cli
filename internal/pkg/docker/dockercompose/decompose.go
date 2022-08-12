package dockercompose

import (
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"sort"
)

func decomposeService(content []byte, svcName string) (*manifest.BackendServiceConfig, IgnoredKeys, error) {
	config, err := loader.ParseYAML(content)
	if err != nil {
		return nil, nil, err
	}

	svcs, ok := config["services"]
	if !ok {
		return nil, nil, fmt.Errorf("compose file has no services")
	}

	services, ok := svcs.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("\"services\" top-level element was not a map")
	}

	svc, ok := services[svcName]
	if !ok {
		return nil, nil, fmt.Errorf("no service named \"%s\" in this Compose file", svcName)
	}

	service, ok := svc.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("\"services.%s\" element was not a map", svcName)
	}

	var ignored, fatal []string

	for key := range service {
		if _, ok := ignoredServiceKeys[key]; ok {
			ignored = append(ignored, key)
		} else if _, ok := fatalServiceKeys[key]; ok {
			fatal = append(fatal, key)
		}
	}

	// sort so we have consistent (testable) error messages
	sort.Strings(ignored)
	sort.Strings(fatal)

	if len(fatal) != 0 {
		return nil, nil, fmt.Errorf("\"services.%s\" relies on fatally-unsupported Compose keys: %v", svcName, fatal)
	}

	project, err := loader.Load(types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
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
		return nil, nil, fmt.Errorf("get service from Compose project: %w", err)
	}

	// TODO: Port
	var port uint16 = 80
	backendSvc, svcIgnored, err := convertBackendService(&svcConfig, port)

	if err != nil {
		return nil, nil, fmt.Errorf("convert Compose service to Copilot manifest: %w", err)
	}

	for _, ign := range svcIgnored {
		ignored = append(ignored, ign)
	}

	return backendSvc, svcIgnored, nil
}
