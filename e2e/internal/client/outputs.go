package client

import (
	"encoding/json"
	"strings"
)

type SvcShowOutput struct {
	SvcName            string                      `json:"service"`
	Type               string                      `json:"type"`
	AppName            string                      `json:"application"`
	Configs            []SvcShowConfigurations     `json:"configurations"`
	ServiceDiscoveries []SvcShowServiceDiscoveries `json:"serviceDiscovery"`
	Routes             []SvcShowRoutes             `json:"routes"`
	Variables          []SvcShowVariables          `json:"variables"`
}

type SvcShowConfigurations struct {
	Environment string `json:"environment"`
	Port        string `json:"port"`
	Tasks       string `json:"tasks"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
}

type SvcShowRoutes struct {
	Environment string `json:"environment"`
	URL         string `json:"url"`
}

type SvcShowServiceDiscoveries struct {
	Environment []string `json:"environment"`
	Namespace   string   `json:"namespace"`
}

type SvcShowVariables struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

func toSvcShowOutput(jsonInput string) (*SvcShowOutput, error) {
	var output SvcShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

type SvcListOutput struct {
	Services []SvcDescription `json:"services"`
}

type SvcDescription struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	AppName string `json:"app"`
}

func toSvcListOutput(jsonInput string) (*SvcListOutput, error) {
	var output SvcListOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

type SvcLogsOutput struct {
	LogStreamName string `json:"logStreamName"`
	IngestionTime int64  `json:"ingestionTime"`
	Timestamp     int64  `json:"timestamp"`
	Message       string `json:"message"`
}

func toSvcLogsOutput(jsonInput string) ([]SvcLogsOutput, error) {
	output := []SvcLogsOutput{}
	for _, logLine := range strings.Split(strings.TrimSpace(jsonInput), "\n") {
		var parsedLogLine SvcLogsOutput
		if err := json.Unmarshal([]byte(logLine), &parsedLogLine); err != nil {
			return nil, err
		}
		output = append(output, parsedLogLine)
	}
	return output, nil
}

type AppShowOutput struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

func toAppShowOutput(jsonInput string) (*AppShowOutput, error) {
	var output AppShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

type EnvShowOutput struct {
	Environment EnvDescription    `json:"environment"`
	Services    []EnvShowServices `json:"services"`
	Tags        map[string]string `json:"tags"`
}

type EnvShowServices struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func toEnvShowOutput(jsonInput string) (*EnvShowOutput, error) {
	var output EnvShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

type EnvListOutput struct {
	Envs []EnvDescription `json:"environments"`
}

type EnvDescription struct {
	Name          string `json:"name"`
	App           string `json:"app"`
	Region        string `json:"region"`
	Account       string `json:"accountID"`
	Prod          bool   `json:"prod"`
	RegistryURL   string `json:"registryURL"`
	ExecutionRole string `json:"executionRoleARN"`
	ManagerRole   string `json:"managerRoleARN"`
}

func toEnvListOutput(jsonInput string) (*EnvListOutput, error) {
	var output EnvListOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}
