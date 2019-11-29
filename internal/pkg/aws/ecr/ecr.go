// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecr contains utility functions for dealing with ECR repos
package ecr

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
)

const (
	urlFmtString      = "%s.dkr.ecr.%s.amazonaws.com/%s"
	arnResourcePrefix = "repository/"
	batchDeleteLimit  = 100
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
		return "", fmt.Errorf("ecr describe repository %s: %w", name, err)
	}

	foundRepositories := result.Repositories

	if len(foundRepositories) <= 0 {
		return "", errors.New("no repositories found")
	}

	repo := result.Repositories[0]

	return *repo.RepositoryUri, nil
}

// Image houses metadata for ECR repository images.
type Image struct {
	Digest string
}

func (i Image) imageIdentifier() *ecr.ImageIdentifier {
	return &ecr.ImageIdentifier{
		ImageDigest: aws.String(i.Digest),
	}
}

// ListImages calls the ECR DescribeImages API and returns a list of
// Image metadata for images in the input ECR repository name.
func (s Service) ListImages(repoName string) ([]Image, error) {
	var images []Image
	nextTokenExist := false
	var resp *ecr.DescribeImagesOutput
	var err error
	for {
		if nextTokenExist {
			resp, err = s.ecr.DescribeImages(&ecr.DescribeImagesInput{
				RepositoryName: aws.String(repoName),
				NextToken:      resp.NextToken,
			})
		} else {
			resp, err = s.ecr.DescribeImages(&ecr.DescribeImagesInput{
				RepositoryName: aws.String(repoName),
			})
			nextTokenExist = true
		}
		if err != nil {
			return nil, fmt.Errorf("ecr repo %s describe images: %w", repoName, err)
		}
		for _, imageDetails := range resp.ImageDetails {
			images = append(images, Image{
				Digest: *imageDetails.ImageDigest,
			})
		}
		if resp.NextToken == nil {
			break
		}
	}
	return images, nil
}

// DeleteImages calls the ECR BatchDeleteImage API with the input image list and repository name.
func (s Service) DeleteImages(images []Image, repoName string) error {
	if len(images) == 0 {
		return nil
	}

	var imageIdentifiers [][]*ecr.ImageIdentifier
	for ind, image := range images {
		if ind%batchDeleteLimit == 0 {
			imageIdentifiers = append(imageIdentifiers, make([]*ecr.ImageIdentifier, 0))
		}
		imageIdentifiers[ind/batchDeleteLimit] = append(imageIdentifiers[ind/batchDeleteLimit], image.imageIdentifier())
	}

	for _, identifiers := range imageIdentifiers {
		resp, err := s.ecr.BatchDeleteImage(&ecr.BatchDeleteImageInput{
			RepositoryName: aws.String(repoName),
			ImageIds:       identifiers,
		})
		if resp != nil {
			for _, failure := range resp.Failures {
				log.Warningf("failed to delete %s:%s : %s %s\n", failure.ImageId.ImageDigest, failure.ImageId.ImageTag, failure.FailureCode, failure.FailureReason)
			}
		}
		if err != nil {
			return fmt.Errorf("ecr repo %s batch delete image: %w", repoName, err)
		}
	}

	return nil
}

// ClearRepository orchestrates a ListImages call followed by a DeleteImages
// call to delete all images from the input ECR repository name.
func (s Service) ClearRepository(repoName string) error {
	images, err := s.ListImages(repoName)

	if err != nil {
		return err
	}

	// TODO: add retry handling in case images are added to a repository after a call to ListImages
	return s.DeleteImages(images, repoName)
}

// URIFromARN converts an ECR Repo ARN to a Repository URI
func URIFromARN(repositoryARN string) (string, error) {
	repoARN, err := arn.Parse(repositoryARN)
	if err != nil {
		return "", fmt.Errorf("parsing repository ARN %s: %w", repositoryARN, err)
	}
	// Repo ARNs look like arn:aws:ecr:region:012345678910:repository/test
	// so we have to strip the repository out.
	repoName := strings.TrimPrefix(repoARN.Resource, arnResourcePrefix)
	return fmt.Sprintf(urlFmtString,
		repoARN.AccountID,
		repoARN.Region,
		repoName), nil
}
