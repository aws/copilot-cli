// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecr provides a client to make API requests to Amazon EC2 Container Registry.
package ecr

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

const (
	urlFmtString      = "%s.dkr.ecr.%s.amazonaws.com/%s"
	arnResourcePrefix = "repository/"
	batchDeleteLimit  = 100
)

type api interface {
	DescribeImages(*ecr.DescribeImagesInput) (*ecr.DescribeImagesOutput, error)
	GetAuthorizationToken(*ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error)
	DescribeRepositories(*ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error)
	BatchDeleteImage(*ecr.BatchDeleteImageInput) (*ecr.BatchDeleteImageOutput, error)
}

// ECR wraps an AWS ECR client.
type ECR struct {
	client api
}

// New returns a ECR configured against the input session.
func New(s *session.Session) ECR {
	return ECR{
		client: ecr.New(s),
	}
}

// Auth represent basic authentication credentials.
type Auth struct {
	Username string
	Password string
}

// GetECRAuth returns the basic authentication credentials needed to push images.
func (c ECR) GetECRAuth() (Auth, error) {
	response, err := c.client.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})

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
func (c ECR) GetRepository(name string) (string, error) {
	result, err := c.client.DescribeRepositories(&ecr.DescribeRepositoriesInput{
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
func (c ECR) ListImages(repoName string) ([]Image, error) {
	var images []Image
	resp, err := c.client.DescribeImages(&ecr.DescribeImagesInput{
		RepositoryName: aws.String(repoName),
	})
	if err != nil {
		return nil, fmt.Errorf("ecr repo %s describe images: %w", repoName, err)
	}
	for _, imageDetails := range resp.ImageDetails {
		images = append(images, Image{
			Digest: *imageDetails.ImageDigest,
		})
	}
	for resp.NextToken != nil {
		resp, err = c.client.DescribeImages(&ecr.DescribeImagesInput{
			RepositoryName: aws.String(repoName),
			NextToken:      resp.NextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("ecr repo %s describe images: %w", repoName, err)
		}
		for _, imageDetails := range resp.ImageDetails {
			images = append(images, Image{
				Digest: *imageDetails.ImageDigest,
			})
		}
	}
	return images, nil
}

// DeleteImages calls the ECR BatchDeleteImage API with the input image list and repository name.
func (c ECR) DeleteImages(images []Image, repoName string) error {
	if len(images) == 0 {
		return nil
	}

	var imageIdentifiers []*ecr.ImageIdentifier
	for _, image := range images {
		imageIdentifiers = append(imageIdentifiers, image.imageIdentifier())
	}
	var imageIdentifiersBatch [][]*ecr.ImageIdentifier
	for batchDeleteLimit < len(imageIdentifiers) {
		imageIdentifiers, imageIdentifiersBatch = imageIdentifiers[batchDeleteLimit:], append(imageIdentifiersBatch, imageIdentifiers[0:batchDeleteLimit])
	}
	imageIdentifiersBatch = append(imageIdentifiersBatch, imageIdentifiers)
	for _, identifiers := range imageIdentifiersBatch {
		resp, err := c.client.BatchDeleteImage(&ecr.BatchDeleteImageInput{
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
func (c ECR) ClearRepository(repoName string) error {
	images, err := c.ListImages(repoName)

	if err == nil {
		// TODO: add retry handling in case images are added to a repository after a call to ListImages
		return c.DeleteImages(images, repoName)
	}
	if isRepoNotFoundErr(errors.Unwrap(err)) {
		return nil
	}
	return err
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

func isRepoNotFoundErr(err error) bool {
	aerr, ok := err.(awserr.Error)
	if !ok {
		return false
	}
	if aerr.Code() == "RepositoryNotFoundException" {
		return true
	}
	return false
}
