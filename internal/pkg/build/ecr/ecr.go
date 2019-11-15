// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecr wraps AWS Elastic Container Registry (ECR) functionality.
package ecr

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
)

// Service wraps an AWS ECR client.
type Service struct {
	ecr ecriface.ECRAPI
}

// New returns a Service configured against the input session.
func New(s *session.Session) Service {
	return Service{
		ecr: ecr.New(s),
	}
}

// Auth represent basic authentication credentials.
type Auth struct {
	Username string
	Password string
}

// GetECRAuth returns the basic authentication credentials needed to push images.
func (s Service) GetECRAuth() (Auth, error) {
	response, err := s.ecr.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})

	if err != nil {
		return Auth{}, fmt.Errorf("get ECR auth: %w", err)
	}

	authToken, err := base64.StdEncoding.DecodeString(*response.AuthorizationData[0].AuthorizationToken)

	if err != nil {
		return Auth{}, fmt.Errorf("decode auth token: %w", err)
	}

	tokenStrings := strings.Split(string(authToken), ":")

	return Auth{
		Username: tokenStrings[0],
		Password: tokenStrings[1],
	}, nil
}

// GetRepository returns the ECR repository URI.
func (s Service) GetRepository(name string) (string, error) {
	result, err := s.ecr.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RepositoryNames: aws.StringSlice([]string{name}),
	})

	if err != nil {
		return "", fmt.Errorf("repository %s not found: %w", name, err)
	}

	foundRepositories := result.Repositories

	if len(foundRepositories) <= 0 {
		return "", errors.New("no repositories found")
	}

	repo := result.Repositories[0]

	return *repo.RepositoryUri, nil
}
