// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package secretsmanager wraps AWS SecretsManager API functionality.
package secretsmanager

import (
	"fmt"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

type SecretsManagerAPI interface {
	CreateSecret(*secretsmanager.CreateSecretInput) (*secretsmanager.CreateSecretOutput, error)
	DeleteSecret(*secretsmanager.DeleteSecretInput) (*secretsmanager.DeleteSecretOutput, error)
}

// SecretsManager is in charge of fetching and creating projects, environment
// and pipeline configuration in SecretsManager.
type SecretsManager struct {
	secretsManager SecretsManagerAPI
	sessionRegion  string
}

// NewSecretsManager returns a SecretsManager configured with the input session.
func NewStore() (*SecretsManager, error) {
	p := session.NewProvider()
	sess, err := p.Default()

	if err != nil {
		return nil, err
	}

	return &SecretsManager{
		secretsManager: secretsmanager.New(sess),
		sessionRegion:  *sess.Config.Region,
	}, nil
}

var secretTags = func() []*secretsmanager.Tag {
	timestamp := time.Now().UTC().Format(time.UnixDate)
	return []*secretsmanager.Tag{
		{
			Key:   aws.String("ecs-project"),
			Value: aws.String(timestamp),
		},
	}
}

// CreateSecret creates a secret and returns secret ARN
// NOTE: Currently the default KMS key ("aws/secretsmanager") is used for
// encrypting the secret.
func (s *SecretsManager) CreateSecret(secretName, secretString string) (string, error) {
	resp, err := s.secretsManager.CreateSecret(&secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(secretString),
		Tags:         secretTags(),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == secretsmanager.ErrCodeResourceExistsException {
				// TODO update secret if value provided?
				return "", &ErrSecretAlreadyExists{
					secretName: secretName,
					parentErr:  err,
				}
			}
		}
		return "", fmt.Errorf("create secret %s: %w", secretName, err)

	}

	return aws.StringValue(resp.ARN), nil
}

func (s *SecretsManager) DeleteSecret(secretName string) error {
	_, err := s.secretsManager.DeleteSecret(&secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(secretName),
		ForceDeleteWithoutRecovery: aws.Bool(true), // forego the waiting period to delete the secret
	})

	if err != nil {
		return fmt.Errorf("delete secret %s from secrets manager: %+v", secretName, err)
	}
	return nil
}

type ErrSecretAlreadyExists struct {
	secretName string
	parentErr  error
}

func (err *ErrSecretAlreadyExists) Error() string {
	return fmt.Sprintf("secret %s already exists", err.secretName)
}
