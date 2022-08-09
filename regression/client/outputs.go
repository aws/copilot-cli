// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
)

type wkldDescription struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	AppName string `json:"app"`
}

// SvcListOutput contains summaries of services.
type SvcListOutput struct {
	Services []wkldDescription `json:"services"`
}

// ToSvcListOutput unmarshal a JSON string to a SvcListOutput struct.
func ToSvcListOutput(jsonInput string) (*SvcListOutput, error) {
	var output SvcListOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

// SvcShowOutput contains detailed information about a service.
type SvcShowOutput struct {
	SvcName            string                            `json:"service"`
	Type               string                            `json:"type"`
	AppName            string                            `json:"application"`
	Configs            []svcShowConfigurations           `json:"configurations"`
	ServiceDiscoveries []svcShowServiceDiscoveries       `json:"serviceDiscovery"`
	Routes             []svcShowRoutes                   `json:"routes"`
	Variables          []svcShowVariables                `json:"variables"`
	Resources          map[string][]*svcShowResourceInfo `json:"resources"`
}

type svcShowConfigurations struct {
	Environment string `json:"environment"`
	Port        string `json:"port"`
	Tasks       string `json:"tasks"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
}

type svcShowRoutes struct {
	Environment string `json:"environment"`
	URL         string `json:"url"`
}

type svcShowServiceDiscoveries struct {
	Environment []string `json:"environment"`
	Namespace   string   `json:"namespace"`
}

type svcShowVariables struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

type svcShowResourceInfo struct {
	Type       string `json:"type"`
	PhysicalID string `json:"physicalID"`
}

// ToSvcShowOutput unmarshal a JSON string to a SvcShowOutput struct.
func ToSvcShowOutput(jsonInput string) (*SvcShowOutput, error) {
	var output SvcShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

// JobListOutput contains summaries of jobs. 
type JobListOutput struct {
	Jobs []wkldDescription `json:"jobs"`
}

// ToJobListOutput unmarshal a JSON string to a JobListOutput struct.
func ToJobListOutput(jsonInput string) (*JobListOutput, error) {
	var output JobListOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}
