package client

import (
	"encoding/json"
	"strings"
)

type AppShowOutput struct {
	AppName   string                  `json:"appName"`
	Type      string                  `json:"type"`
	Project   string                  `json:"project"`
	Configs   []AppShowConfigurations `json:"configurations"`
	Routes    []AppShowRoutes         `json:"routes"`
	Variables []AppShowVariables      `json:"variables"`
}

type AppShowConfigurations struct {
	Environment string `json:"environment"`
	Port        string `json:"port"`
	Tasks       string `json:"tasks"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
}

type AppShowRoutes struct {
	Environment string `json:"environment"`
	URL         string `json:"url"`
	Path        string `json:"path"`
}

type AppShowVariables struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

func toAppShowOutput(jsonInput string) (*AppShowOutput, error) {
	var output AppShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

type AppListOutput struct {
	Apps []AppDescription `json:"applications"`
}

type AppDescription struct {
	AppName string `json:"name"`
	Type    string `json:"type"`
	Project string `json:"project"`
}

func toAppListOutput(jsonInput string) (*AppListOutput, error) {
	var output AppListOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

type AppLogsOutput struct {
	TaskID        string `json:"taskID"`
	IngestionTime int64  `json:"ingestionTime"`
	Timestamp     int64  `json:"timestamp"`
	Message       string `json:"message"`
}

func toAppLogsOutput(jsonInput string) ([]AppLogsOutput, error) {
	output := []AppLogsOutput{}
	for _, logLine := range strings.Split(strings.TrimSpace(jsonInput), "\n") {
		var parsedLogLine AppLogsOutput
		if err := json.Unmarshal([]byte(logLine), &parsedLogLine); err != nil {
			return nil, err
		}
		output = append(output, parsedLogLine)
	}
	return output, nil
}

type ProjectShowOutput struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

func toProjectShowOutput(jsonInput string) (*ProjectShowOutput, error) {
	var output ProjectShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

type EnvListOutput struct {
	Envs []EnvDescription `json:"environments"`
}

type EnvDescription struct {
	Name          string `json:"name"`
	Project       string `json:"project"`
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
