// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package apprunner provides a client to retrieve Copilot App Runner information.
package apprunner

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	awsapprunner "github.com/aws/aws-sdk-go/service/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

const (
	serviceResourceType = "apprunner:service"
)

type appRunnerClient interface {
	DescribeOperation(operationId, svcARN string) (*awsapprunner.OperationSummary, error)
	StartDeployment(svcARN string) (string, error)
	DescribeService(svcARN string) (*apprunner.Service, error)
	WaitForOperation(operationId, svcARN string) error
}

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error)
}

// Client retrieves Copilot information from App Runner endpoint.
type Client struct {
	appRunnerClient appRunnerClient
	rgGetter        resourceGetter
}

// New inits a new Client.
func New(sess *session.Session) *Client {
	return &Client{
		rgGetter:        resourcegroups.New(sess),
		appRunnerClient: apprunner.New(sess),
	}
}

// ForceUpdateService forces a new update for an App Runner service given Copilot service info.
func (c Client) ForceUpdateService(app, env, svc string) error {
	svcARN, err := c.serviceARN(app, env, svc)
	if err != nil {
		return err
	}
	id, err := c.appRunnerClient.StartDeployment(svcARN)
	if err != nil {
		return err
	}
	return c.appRunnerClient.WaitForOperation(id, svcARN)
}

// LastUpdatedAt returns the last updated time of the app runner service.
func (c Client) LastUpdatedAt(app, env, svc string) (time.Time, error) {
	svcARN, err := c.serviceARN(app, env, svc)
	if err != nil {
		return time.Time{}, err
	}
	desc, err := c.appRunnerClient.DescribeService(svcARN)
	if err != nil {
		return time.Time{}, fmt.Errorf("describe service: %w", err)
	}
	return desc.DateUpdated, nil
}

func (c Client) serviceARN(app, env, svc string) (string, error) {
	services, err := c.rgGetter.GetResourcesByTags(serviceResourceType, map[string]string{
		deploy.AppTagKey:     app,
		deploy.EnvTagKey:     env,
		deploy.ServiceTagKey: svc,
	})
	if err != nil {
		return "", fmt.Errorf("get App Runner service with tags (%s, %s, %s): %w", app, env, svc, err)
	}
	if len(services) == 0 {
		return "", fmt.Errorf("no App Runner service found for %s in environment %s", svc, env)
	}
	if len(services) > 1 {
		return "", fmt.Errorf("more than one App Runner service with the name %s found in environment %s", svc, env)
	}
	return services[0].ARN, nil
}
