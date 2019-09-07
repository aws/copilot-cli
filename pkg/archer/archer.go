// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Archer application
type Archer struct {
	envStore  EnvironmentStore
	projStore ProjectStore
}

// New returns a new archer applicaiton.
func New(envStore EnvironmentStore, projStore ProjectStore) (*Archer, error) {
	return &Archer{
		envStore:  envStore,
		projStore: projStore,
	}, nil
}
