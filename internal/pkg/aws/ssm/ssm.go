// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ssm provides a client to make API requests to Amazon Systems Manager.
package ssm

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type api interface {
	PutParameter(*ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
	AddTagsToResource(*ssm.AddTagsToResourceInput) (*ssm.AddTagsToResourceOutput, error)
	GetParameterWithContext(context.Context, *ssm.GetParameterInput, ...request.Option) (*ssm.GetParameterOutput, error)
}

// SSM wraps an AWS SSM client.
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

// PutSecret tries to create the secret, and overwrites it if the secret exists and that `Overwrite` is true.
// ErrParameterAlreadyExists is returned if the secret exists and `Overwrite` is false.
func (s *SSM) PutSecret(in PutSecretInput) (*PutSecretOutput, error) {
	// First try to create the secret with the tags.
	out, err := s.createSecret(in)
	if err == nil {
		return out, nil
	}

	// If the parameter already exists and we want to overwrite, we try to overwrite it.
	var errParameterExists *ErrParameterAlreadyExists
	if errors.As(err, &errParameterExists) && in.Overwrite {
		return s.overwriteSecret(in)
	}
	return nil, err
}

// GetSecretValue retrieves the value of a parameter from AWS Systems Manager Parameter Store.
// It takes the name of the parameter as input and returns the corresponding value as a string.
func (s *SSM) GetSecretValue(ctx context.Context, name string) (string, error) {
	resp, err := s.client.GetParameterWithContext(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("get parameter %q from SSM: %w", name, err)
	}
	return aws.StringValue(resp.Parameter.Value), nil
}

func (s *SSM) createSecret(in PutSecretInput) (*PutSecretOutput, error) {
	// Create a secret while adding the tags in a single call instead of separate calls to `PutParameter` and
	// `AddTagsToResource` so that there won't be a case where the parameter is created while the tags are not added.

	tags := convertTags(in.Tags)

	input := &ssm.PutParameterInput{
		DataType: aws.String("text"),
		Type:     aws.String("SecureString"),
		Name:     aws.String(in.Name),
		Value:    aws.String(in.Value),
		Tags:     tags,
	}
	output, err := s.client.PutParameter(input)
	if err == nil {
		return (*PutSecretOutput)(output), nil
	}

	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == ssm.ErrCodeParameterAlreadyExists {
			return nil, &ErrParameterAlreadyExists{in.Name}
		}
	}
	return nil, fmt.Errorf("create parameter %s: %w", in.Name, err)
}

func (s *SSM) overwriteSecret(in PutSecretInput) (*PutSecretOutput, error) {
	// SSM API does not allow `Overwrite` to be true while `Tags` are not nil, so we have to overwrite the resource and
	// add the tags in two separate calls.

	input := &ssm.PutParameterInput{
		DataType:  aws.String("text"),
		Type:      aws.String("SecureString"),
		Name:      aws.String(in.Name),
		Value:     aws.String(in.Value),
		Overwrite: aws.Bool(in.Overwrite),
	}
	output, err := s.client.PutParameter(input)
	if err != nil {
		return nil, fmt.Errorf("update parameter %s: %w", in.Name, err)
	}

	tags := convertTags(in.Tags)
	_, err = s.client.AddTagsToResource(&ssm.AddTagsToResourceInput{
		ResourceType: aws.String(ssm.ResourceTypeForTaggingParameter),
		ResourceId:   aws.String(in.Name),
		Tags:         tags,
	})
	if err != nil {
		return nil, fmt.Errorf("add tags to resource %s: %w", in.Name, err)
	}
	return (*PutSecretOutput)(output), nil
}

func convertTags(inTags map[string]string) []*ssm.Tag {
	// Sort the map so that the unit test won't be flaky.
	keys := make([]string, 0, len(inTags))
	for k := range inTags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var tags []*ssm.Tag
	for _, key := range keys {
		tags = append(tags, &ssm.Tag{
			Key:   aws.String(key),
			Value: aws.String(inTags[key]),
		})
	}
	return tags
}
