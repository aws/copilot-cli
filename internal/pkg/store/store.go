// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/*
Package store implements CRUD operations for project, environment, application and
pipeline configuration. This configuration contains the archer projects
a customer has, and the environments and pipelines associated with each
project.
*/
package store

import (
	"encoding/json"
	"log"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/route53"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	route53API "github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

// Parameter name formats for resources in a project. Projects are laid out in SSM
// based on path - each parameter's key has a certain format, and you can have
// hierarchies based on that format. Projects are at the root of the hierarchy.
// Searching SSM for all parameters with the `rootProjectPath` key will give you
// all the project keys, for example.

// current schema Version for Projects
const schemaVersion = "1.0"

// schema formats supported in current schemaVersion. NOTE: May change to map in the future.
const (
	rootProjectPath  = "/ecs-cli-v2/"
	fmtProjectPath   = "/ecs-cli-v2/%s"
	rootEnvParamPath = "/ecs-cli-v2/%s/environments/"
	fmtEnvParamPath  = "/ecs-cli-v2/%s/environments/%s" // path for an environment in a project
	rootAppParamPath = "/ecs-cli-v2/%s/applications/"
	fmtAppParamPath  = "/ecs-cli-v2/%s/applications/%s" // path for an application in a project
)

type identityService interface {
	Get() (identity.Caller, error)
}

// Store is in charge of fetching and creating projects, environment and pipeline configuration in SSM.
type Store struct {
	idClient      identityService
	route53Svc    route53.Lister
	route53Full   route53iface.Route53API
	rdsClient     rdsiface.RDSAPI
	ssmClient     ssmiface.SSMAPI
	sessionRegion string
}

// New returns a Store allowing you to query or create Projects or Environments.
func New() (*Store, error) {
	p := session.NewProvider()
	sess, err := p.Default()

	if err != nil {
		return nil, err
	}

	return &Store{
		idClient: identity.New(sess),
		// See https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/DNSLimitations.html#limits-service-quotas
		// > To view limits and request higher limits for Route 53, you must change the Region to US East (N. Virginia).
		// So we have to set the region to us-east-1 to be able to find out if a domain name exists in the account.
		route53Svc:    route53API.New(sess, aws.NewConfig().WithRegion("us-east-1")),
		route53Full:   route53API.New(sess, aws.NewConfig().WithRegion("us-east-1")),
		rdsClient:     rds.New(sess),
		ssmClient:     ssm.New(sess),
		sessionRegion: *sess.Config.Region,
	}, nil
}

func (s *Store) listParams(path string) ([]*string, error) {
	var serializedParams []*string

	var nextToken *string
	for {
		params, err := s.ssmClient.GetParametersByPath(&ssm.GetParametersByPathInput{
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
	identity, err := s.idClient.Get()
	region := s.sessionRegion
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
