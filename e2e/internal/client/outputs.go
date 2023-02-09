// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
	"strings"
	"time"
)

// SvcStatusOutput is the JSON output of the svc status.
type SvcStatusOutput struct {
	Status    string `json:"status"`
	Service   SvcStatusServiceInfo
	Tasks     []SvcStatusTaskInfo  `json:"tasks"`
	Alarms    []SvcStatusAlarmInfo `json:"alarms"`
	LogEvents []*SvcLogsOutput     `json:"logEvents"`
}

// SvcStatusServiceInfo contains the status info of a service.
type SvcStatusServiceInfo struct {
	DesiredCount     int64     `json:"desiredCount"`
	RunningCount     int64     `json:"runningCount"`
	Status           string    `json:"status"`
	LastDeploymentAt time.Time `json:"lastDeploymentAt"`
	TaskDefinition   string    `json:"taskDefinition"`
}

// Image contains very basic info of a container image.
type Image struct {
	ID     string
	Digest string
}

// SvcStatusTaskInfo contains the status info of a task.
type SvcStatusTaskInfo struct {
	Health        string    `json:"health"`
	ID            string    `json:"id"`
	Images        []Image   `json:"images"`
	LastStatus    string    `json:"lastStatus"`
	StartedAt     time.Time `json:"startedAt"`
	StoppedAt     time.Time `json:"stoppedAt"`
	StoppedReason string    `json:"stoppedReason"`
}

// SvcStatusAlarmInfo contains CloudWatch alarm status info.
type SvcStatusAlarmInfo struct {
	Arn          string    `json:"arn"`
	Name         string    `json:"name"`
	Reason       string    `json:"reason"`
	Status       string    `json:"status"`
	Type         string    `json:"type"`
	UpdatedTimes time.Time `json:"updatedTimes"`
}

func toSvcStatusOutput(jsonInput string) (*SvcStatusOutput, error) {
	var output SvcStatusOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

// SvcShowOutput is the JSON output of the svc show.
type SvcShowOutput struct {
	SvcName            string                            `json:"service"`
	Type               string                            `json:"type"`
	AppName            string                            `json:"application"`
	Configs            []SvcShowConfigurations           `json:"configurations"`
	ServiceDiscoveries []SvcShowServiceEndpoints         `json:"serviceDiscovery"`
	ServiceConnects    []SvcShowServiceEndpoints         `json:"serviceConnect"`
	Routes             []SvcShowRoutes                   `json:"routes"`
	Variables          []SvcShowVariables                `json:"variables"`
	Resources          map[string][]*SvcShowResourceInfo `json:"resources"`
	Secrets            []SvcShowSecrets                  `json:"secrets"`
}

// SvcShowConfigurations contains serialized configuration parameters for a service.
type SvcShowConfigurations struct {
	Environment string `json:"environment"`
	Port        string `json:"port"`
	Tasks       string `json:"tasks"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
}

// SvcShowRoutes contains serialized route parameters for a web service.
type SvcShowRoutes struct {
	Environment string `json:"environment"`
	URL         string `json:"url"`
	Ingress     string `json:"ingress"`
}

// SvcShowServiceEndpoints contains serialized endpoint info for a service.
type SvcShowServiceEndpoints struct {
	Environment []string `json:"environment"`
	Endpoint    string   `json:"endpoint"`
}

// SvcShowVariables contains serialized environment variables for a service.
type SvcShowVariables struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

// SvcShowSecrets contains serialized secrets for a service.
type SvcShowSecrets struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

// SvcShowResourceInfo contains serialized resource info for a service.
type SvcShowResourceInfo struct {
	Type       string `json:"type"`
	PhysicalID string `json:"physicalID"`
}

func toSvcShowOutput(jsonInput string) (*SvcShowOutput, error) {
	var output SvcShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

// SvcListOutput is the JSON output for svc list.
type SvcListOutput struct {
	Services []WkldDescription `json:"services"`
}

// WkldDescription contains the brief description of the workload.
type WkldDescription struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	AppName string `json:"app"`
}

func toSvcListOutput(jsonInput string) (*SvcListOutput, error) {
	var output SvcListOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

// JobListOutput is the JSON output for job list.
type JobListOutput struct {
	Jobs []WkldDescription `json:"jobs"`
}

func toJobListOutput(jsonInput string) (*JobListOutput, error) {
	var output JobListOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

// SvcLogsOutput is the JSON output of svc logs.
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

// AppShowOutput is the JSON output of app show.
type AppShowOutput struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

func toAppShowOutput(jsonInput string) (*AppShowOutput, error) {
	var output AppShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

// EnvShowOutput is the JSON output of env show.
type EnvShowOutput struct {
	Environment EnvDescription      `json:"environment"`
	Services    []EnvShowServices   `json:"services"`
	Tags        map[string]string   `json:"tags"`
	Resources   []map[string]string `json:"resources"`
}

// EnvShowServices contains brief info about a service.
type EnvShowServices struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func toEnvShowOutput(jsonInput string) (*EnvShowOutput, error) {
	var output EnvShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

// EnvListOutput is the JSON output of env list.
type EnvListOutput struct {
	Envs []EnvDescription `json:"environments"`
}

// EnvDescription contains descriptive info about an environment.
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

// PipelineShowOutput represents the JSON output of the "pipeline show" command.
type PipelineShowOutput struct {
	Name   string `json:"name"`
	Stages []struct {
		Name     string `json:"name"`
		Category string `json:"category"`
	} `json:"stages"`
}

// PipelineStatusOutput represents the JSON output of the "pipeline status" command.
type PipelineStatusOutput struct {
	States []struct {
		Name    string `json:"stageName"`
		Actions []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"actions"`
	} `json:"stageStates"`
}
