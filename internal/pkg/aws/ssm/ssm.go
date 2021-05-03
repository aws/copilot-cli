// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ssm provides a client to make API requests to Amazon Systems Manager.
package ssm

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type api interface {
	PutParameter(input *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
}

type SSM struct {
	client api
}

// New returns a SSM service configured against the input session.
func New(s *session.Session) *SSM {
	return &SSM{
		client: ssm.New(s),
	}
}

// PutSecretInput contains fields needed to create or update a secret.
type PutSecretInput struct {
	Name      string
	Value     string
	Overwrite bool
	Tags      map[string]string
}

// PutSecret creates or updates a SecureString parameter.
func (s *SSM) PutSecret(in PutSecretInput) error {
	tags := make([]*ssm.Tag, 0)
	for key, value := range in.Tags {
		tags = append(tags, &ssm.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	input := &ssm.PutParameterInput{
		DataType:  aws.String("text"),
		Type:      aws.String("SecureString"),
		Name:      aws.String(in.Name),
		Value:     aws.String(in.Value),
		Overwrite: aws.Bool(in.Overwrite),
		Tags:      tags,
	}
	_, err := s.client.PutParameter(input)
	if err != nil {
		return fmt.Errorf("put parameter %s: %w", in.Name, err)
	}
	return nil
}
