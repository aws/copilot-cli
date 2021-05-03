// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ssm provides a client to make API requests to Amazon Systems Manager.
package ssm

import (
	"fmt"
	"sort"

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

// PutSecretOutput wraps an ssm PutParameterOutput struct.
type PutSecretOutput ssm.PutParameterOutput

// PutSecret creates or updates a SecureString parameter.
func (s *SSM) PutSecret(in PutSecretInput) (*PutSecretOutput, error) {
	tags := make([]*ssm.Tag, 0, len(in.Tags))

	// Sort the map so that the unit test won't be flaky.
	keys := make([]string, 0, len(in.Tags))
	for k := range in.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		tags = append(tags, &ssm.Tag{
			Key:   aws.String(key),
			Value: aws.String(in.Tags[key]),
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
	output, err := s.client.PutParameter(input)
	if err != nil {
		return nil, fmt.Errorf("put parameter %s: %w", in.Name, err)
	}
	return (*PutSecretOutput)(output), nil
}
