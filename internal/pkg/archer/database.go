// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Database represents a serverless Aurora cluster.
type Database struct {
	ClusterIdentifier string `json:"clusterID"`
	DatabaseName      string `json:"dbName"`
	Username          string `json:"username"`
	Password          string `json:"password"`

	Engine      string `json:"engine"`
	MinCapacity int64 `json:"minCapacity"`
	MaxCapacity int64 `json:"maxCapacity"`
}
