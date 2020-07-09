// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package secretsmanager provides a client to make API requests to AWS Secrets Manager.
package secretsmanager

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
)

type api interface {
	CreateSecret(*secretsmanager.CreateSecretInput) (*secretsmanager.CreateSecretOutput, error)
	DeleteSecret(*secretsmanager.DeleteSecretInput) (*secretsmanager.DeleteSecretOutput, error)
}

// SecretsManager wraps the AWS SecretManager client.
type SecretsManager struct {
	secretsManager api
	sessionRegion  string
}

// New returns a SecretsManager configured with the default session.
func New() (*SecretsManager, error) {
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
			Key:   aws.String("copilot-application"),
			Value: aws.String(timestamp),
		},
	}
}

// CreateSecret creates a secret using the default KMS key "aws/secretmanager" to encrypt the secret and returns its ARN.
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

// DeleteSecret force removes the secret from SecretsManager.
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

// ErrSecretAlreadyExists occurs if a secret with the same name already exists.
type ErrSecretAlreadyExists struct {
	secretName string
	parentErr  error
}

func (err *ErrSecretAlreadyExists) Error() string {
	return fmt.Sprintf("secret %s already exists", err.secretName)
}
