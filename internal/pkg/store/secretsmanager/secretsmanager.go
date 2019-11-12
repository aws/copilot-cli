// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package secretsmanager wraps AWS SecretsManager API functionality.
package secretsmanager

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
)

// SecretsManager is in charge of fetching and creating projects, environment and pipeline
// configuration in SecretsManager.
type SecretsManager struct {
	secretsManager secretsmanageriface.SecretsManagerAPI
	sessionRegion  string
}

// NewSecretsManager returns a SecretsManager configured with the input session.
func NewStore() (*SecretsManager, error) {
	sess, err := session.Default()

	if err != nil {
		return nil, err
	}

	return &SecretsManager{
		secretsManager: secretsmanager.New(sess),
		sessionRegion:  *sess.Config.Region,
	}, nil
}

// CreateSecret creates a secret and returns secretn ARN
// NOTE: Currently the default KMS key ("aws/secretsmanager") is used for
// encrypting the secret.
func (s *SecretsManager) CreateSecret(secretName, secretString string) (string, error) {
	resp, err := s.secretsManager.CreateSecret(&secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(secretString),
		// TODO add Tags/Description?
	})
	if err != nil {
		return "", err
	}

	return aws.StringValue(resp.ARN), nil
}
