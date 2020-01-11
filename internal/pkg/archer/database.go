// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

import "github.com/aws/aws-sdk-go/service/rds"

// Database represents a serverless Aurora cluster.
type Database struct {
	ClusterIdentifier string `json:"clusterID"`
	DatabaseName      string `json:"dbName"`
	Username          string `json:"username"`
	Password          string `json:"password"`

	BackupRetentionPeriod int64  `json:"backupRetentionPeriod"`
	Engine                string `json:"engine"`

	MinCapacity int64 `json:"minCapacity"`
	MaxCapacity int64 `json:"maxCapacity"`
}

// Secretsmanager can manage a database
type DatabaseManager interface {
	DatabaseCreator
	DatabaseDeleter
}

// DatabaseCreator creates a database
type DatabaseCreator interface {
	CreateDatabase(db *Database) (*rds.DBCluster, error)
}

// SecretDeleter deletes a database
type DatabaseDeleter interface {
	DeleteDatabase(clusterID, finalSnapshotID string) error
}
