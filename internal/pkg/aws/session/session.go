// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package session provides functions that return AWS sessions to use in the AWS SDK.
package session

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
)

// Default returns a session configured against the "default" AWS profile.
func Default() (*session.Session, error) {
	return session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			CredentialsChainVerboseErrors: aws.Bool(true),
		},
		SharedConfigState: session.SharedConfigEnable,
	})
}

// FromProfile returns a session configured against the input profile name.
func FromProfile(name string) (*session.Session, error) {
	return session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			CredentialsChainVerboseErrors: aws.Bool(true),
		},
		SharedConfigState: session.SharedConfigEnable,
		Profile:           name,
	})
}

// FromRole returns a session configured against the input role and region.
func FromRole(roleARN string, region string) (*session.Session, error) {
	defaultSession, err := Default()

	if err != nil {
		return nil, fmt.Errorf("error creating default session: %w", err)
	}

	creds := stscreds.NewCredentials(defaultSession, roleARN)
	return session.NewSession(&aws.Config{
		CredentialsChainVerboseErrors: aws.Bool(true),
		Credentials:                   creds,
		Region:                        &region,
	})
}
