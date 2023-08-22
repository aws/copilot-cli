// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package secretsmanager provides a client to make API requests to AWS Secrets Manager.
package secretsmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

type api interface {
	CreateSecret(*secretsmanager.CreateSecretInput) (*secretsmanager.CreateSecretOutput, error)
	DeleteSecret(*secretsmanager.DeleteSecretInput) (*secretsmanager.DeleteSecretOutput, error)
	DescribeSecret(*secretsmanager.DescribeSecretInput) (*secretsmanager.DescribeSecretOutput, error)
	GetSecretValueWithContext(context.Context, *secretsmanager.GetSecretValueInput, ...request.Option) (*secretsmanager.GetSecretValueOutput, error)
}

// SecretsManager wraps the AWS SecretManager client.
type SecretsManager struct {
	secretsManager api
	sessionRegion  string
}

// New returns a SecretsManager configured against the input session.
func New(s *session.Session) *SecretsManager {
	return &SecretsManager{
		secretsManager: secretsmanager.New(s),
		sessionRegion:  *s.Config.Region,
	}
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
		return fmt.Errorf("delete secret %s from secrets manager: %w", secretName, err)
	}
	return nil
}

// DescribeSecretOutput is the output returned by DescribeSecret.
type DescribeSecretOutput struct {
	Name        *string
	CreatedDate *time.Time
	Tags        []*secretsmanager.Tag
}

// DescribeSecret retrieves the details of a secret.
func (s *SecretsManager) DescribeSecret(secretName string) (*DescribeSecretOutput, error) {
	resp, err := s.secretsManager.DescribeSecret(&secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == secretsmanager.ErrCodeResourceNotFoundException {
				return nil, &ErrSecretNotFound{
					secretName: secretName,
					parentErr:  err,
				}
			}
		}
		return nil, fmt.Errorf("describe secret %s: %w", secretName, err)
	}

	return &DescribeSecretOutput{
		Name:        resp.Name,
		CreatedDate: resp.CreatedDate,
		Tags:        resp.Tags,
	}, nil
}

// GetSecretValue retrieves the value of a secret from AWS Secrets Manager.
// It takes the name of the secret as input and returns the corresponding value as a string.
func (s *SecretsManager) GetSecretValue(ctx context.Context, name string) (string, error) {
	resp, err := s.secretsManager.GetSecretValueWithContext(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	})
	if err != nil {
		return "", fmt.Errorf("get secret %q from secrets manager: %w", name, err)
	}
	return aws.StringValue(resp.SecretString), nil
}

// ErrSecretAlreadyExists occurs if a secret with the same name already exists.
type ErrSecretAlreadyExists struct {
	secretName string
	parentErr  error
}

func (err *ErrSecretAlreadyExists) Error() string {
	return fmt.Sprintf("secret %s already exists", err.secretName)
}

// ErrSecretNotFound occurs if a secret with the given name does not exist.
type ErrSecretNotFound struct {
	secretName string
	parentErr  error
}

func (err *ErrSecretNotFound) Error() string {
	return fmt.Sprintf("secret %s was not found: %s", err.secretName, err.parentErr)
}
