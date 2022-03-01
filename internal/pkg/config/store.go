// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/*
Package config implements CRUD operations for application, environment, service and
pipeline configuration. This configuration contains the Copilot applications
a customer has, and the environments and pipelines associated with each
application.
*/
package config

import (
	"encoding/json"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
)

// Parameter name formats for resources in an application. Applications are laid out in SSM
// based on path - each parameter's key has a certain format, and you can have
// hierarchies based on that format. Applications are at the root of the hierarchy.
// Searching SSM for all parameters with the `rootApplicationPath` key will give you
// all the application keys, for example.

// current schema Version for Apps.
const schemaVersion = "1.0"

// schema formats supported in current schemaVersion. NOTE: May change to map in the future.
const (
	rootApplicationPath = "/copilot/applications/"
	fmtApplicationPath  = "/copilot/applications/%s"
	rootEnvParamPath    = "/copilot/applications/%s/environments/"
	fmtEnvParamPath     = "/copilot/applications/%s/environments/%s" // path for an environment in an application
	rootWkldParamPath   = "/copilot/applications/%s/components/"
	fmtWkldParamPath    = "/copilot/applications/%s/components/%s" // path for a workload in an application
)

// IAMIdentityGetter is the interface to get information about the IAM user or role whose credentials are used to make AWS requests.
type IAMIdentityGetter interface {
	Get() (identity.Caller, error)
}

// SSM is the interface for the AWS SSM client.
type SSM interface {
	PutParameter(in *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
	GetParametersByPath(in *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)
	GetParameter(in *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
	DeleteParameter(in *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error)
}

// Store is in charge of fetching and creating applications, environment, services and other workloads, and pipeline configuration in SSM.
type Store struct {
	sts       IAMIdentityGetter
	ssm       SSM
	appRegion string
}

// NewSSMStore returns a new store, allowing you to query or create Applications, Environments, Services, and other workloads.
func NewSSMStore(sts IAMIdentityGetter, ssm SSM, appRegion string) *Store {
	return &Store{
		sts:       sts,
		ssm:       ssm,
		appRegion: appRegion,
	}
}

func (s *Store) listParams(path string) ([]*string, error) {
	var serializedParams []*string

	var nextToken *string
	for {
		params, err := s.ssm.GetParametersByPath(&ssm.GetParametersByPathInput{
			Path:      aws.String(path),
			Recursive: aws.Bool(false),
			NextToken: nextToken,
		})

		if err != nil {
			return nil, err
		}

		for _, param := range params.Parameters {
			serializedParams = append(serializedParams, param.Value)
		}

		nextToken = params.NextToken
		if nextToken == nil {
			break
		}
	}
	return serializedParams, nil
}

// Retrieves the caller's Account ID with a best effort. If it fails to fetch the Account ID,
// this returns "unknown".
func (s *Store) getCallerAccountAndRegion() (string, string) {
	identity, err := s.sts.Get()
	region := s.appRegion
	if err != nil {
		log.Printf("Failed to get caller's Account ID %v", err)
		return "unknown", region
	}
	return identity.Account, region
}

func marshal(e interface{}) (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
