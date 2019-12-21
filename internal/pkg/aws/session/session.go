// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package session provides functions that return AWS sessions to use in the AWS SDK.
package session

import (
	"fmt"
	"runtime"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/version"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
)

const userAgentHeader = "User-Agent"

// userAgentHandler returns a http request handler that sets a custom user agent to all aws requests.
func userAgentHandler() request.NamedHandler {
	return request.NamedHandler{
		Name: "UserAgentHandler",
		Fn: func(r *request.Request) {
			userAgent := r.HTTPRequest.Header.Get(userAgentHeader)
			r.HTTPRequest.Header.Set(userAgentHeader,
				fmt.Sprintf("aws-ecs-cli-v2/%s (%s) %s", version.Version, runtime.GOOS, userAgent))
		},
	}
}

// Provider holds methods to create sessions.
type Provider struct{}

// Default returns a session configured against the "default" AWS profile.
func (p *Provider) Default() (*session.Session, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			CredentialsChainVerboseErrors: aws.Bool(true),
		},
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}
	sess.Handlers.Build.PushBackNamed(userAgentHandler())
	return sess, err
}

// DefaultWithRegion returns a session configured against the "default" AWS profile and the input region.
func (p *Provider) DefaultWithRegion(region string) (*session.Session, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, err
	}
	sess.Handlers.Build.PushBackNamed(userAgentHandler())
	return sess, err
}

// FromProfile returns a session configured against the input profile name.
func (p *Provider) FromProfile(name string) (*session.Session, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			CredentialsChainVerboseErrors: aws.Bool(true),
		},
		SharedConfigState: session.SharedConfigEnable,
		Profile:           name,
	})
	if err != nil {
		return nil, err
	}
	sess.Handlers.Build.PushBackNamed(userAgentHandler())
	return sess, err
}

// FromRole returns a session configured against the input role and region.
func (p *Provider) FromRole(roleARN string, region string) (*session.Session, error) {
	defaultSession, err := p.Default()

	if err != nil {
		return nil, fmt.Errorf("error creating default session: %w", err)
	}

	creds := stscreds.NewCredentials(defaultSession, roleARN)
	sess, err := session.NewSession(&aws.Config{
		CredentialsChainVerboseErrors: aws.Bool(true),
		Credentials:                   creds,
		Region:                        &region,
	})
	if err != nil {
		return nil, err
	}
	sess.Handlers.Build.PushBackNamed(userAgentHandler())
	return sess, err
}
