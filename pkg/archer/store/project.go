// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import "encoding/json"

// Project is a named collection of environments.
type Project struct {
	Name    string `json:"name"`    // Name of a project. Must be unique amongst other projects in the same account
	Version string `json:"version"` // The version of the project layout.
}

// Marshal serializes the project into a JSON document and returns it.
// If an error occurred during the serialization, the empty string and the error is returned.
func (e *Project) Marshal() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
