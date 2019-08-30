// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import "encoding/json"

// Environment represents the configuration of a particular Environment in a Project. It includes
// the location of the Environment (account and region), the name of the environment, as well as the project
// the environment belongs to.
type Environment struct {
	Project   string `json:"project"`   // Name of the project this environment belongs to.
	Name      string `json:"name"`      // Name of the environment, must be unique within a project.
	Region    string `json:"region"`    // Name of the region this environment is stored in.
	AccountID string `json:"accountID"` // Account ID of the account this environment is stored in.
	Prod      bool   `json:"prod"`      // Whether or not this environment is a production environment.
}

// Marshal serializes the environment into a JSON document and returns it.
// If an error occurred during the serialization, the empty string and the error is returned.
func (e *Environment) Marshal() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
