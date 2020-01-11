package store

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
)

func (s *Store) CreateDatabase(db *archer.Database) (*rds.DBCluster, error) {
	var engine string
	switch db.Engine {
	case "mysql":
		engine = "aurora"
	case "postgresql":
		engine = "aurora-postgresql"
	}

	output, err := s.rdsClient.CreateDBCluster(&rds.CreateDBClusterInput{
		BackupRetentionPeriod: aws.Int64(db.BackupRetentionPeriod),
		DatabaseName:          aws.String(db.DatabaseName),
		DBClusterIdentifier:   aws.String(db.ClusterIdentifier),
		Engine:                aws.String(engine),
		EngineMode:            aws.String("serverless"),
		MasterUserPassword:    aws.String(db.Password),
		MasterUsername:        aws.String(db.Username),
		ScalingConfiguration: &rds.ScalingConfiguration{
			AutoPause:   aws.Bool(true),
			MaxCapacity: aws.Int64(db.MaxCapacity),
			MinCapacity: aws.Int64(db.MinCapacity),
		},
		StorageEncrypted: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	return output.DBCluster, nil
}

func (s *Store) DeleteDatabase(clusterID, finalSnapshotID string) error {
	_, err := s.rdsClient.DeleteDBCluster(&rds.DeleteDBClusterInput{
		DBClusterIdentifier:       aws.String(clusterID),
		FinalDBSnapshotIdentifier: aws.String(finalSnapshotID),
		SkipFinalSnapshot:         aws.Bool(false),
	})
	return err
}
