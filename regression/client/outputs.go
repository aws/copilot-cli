// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
)

type appShowOutput struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// ToAppShowOutput unmarshal a JSON string to an appShowOutput struct.
func ToAppShowOutput(jsonInput string) (*appShowOutput, error) {
	var output appShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

// WkldDescription describes a workload.
type WkldDescription struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	AppName string `json:"app"`
}

type svcListOutput struct {
	Services []WkldDescription `json:"services"`
}

// ToSvcListOutput unmarshal a JSON string to a svcListOutput struct.
func ToSvcListOutput(jsonInput string) (*svcListOutput, error) {
	var output svcListOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

type svcShowOutput struct {
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

// ToSvcShowOutput unmarshal a JSON string to a svcShowOutput struct.
func ToSvcShowOutput(jsonInput string) (*svcShowOutput, error) {
	var output svcShowOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}

type jobListOutput struct {
	Jobs []WkldDescription `json:"jobs"`
}

// ToJobListOutput unmarshal a JSON string to a jobListOutput struct.
func ToJobListOutput(jsonInput string) (*jobListOutput, error) {
	var output jobListOutput
	return &output, json.Unmarshal([]byte(jsonInput), &output)
}
