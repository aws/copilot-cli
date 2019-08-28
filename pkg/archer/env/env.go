// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package env provides functionality to manage environments.
package env

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

// Environment is a deployment stage.
// An environment has a VPC, ECS cluster, and a load balancer (optional) that your applications share.
type Environment struct {
	Name      string `json:"name"` // Name of the environment, must be unique within a project.
	Region    string `json:"region"`
	AccountID string `json:"accountID"`

	profile string // AWS profile used to retrieve the region and account id.
}

// New creates a new environment using your "default" AWS profile.
// If an error occurs, then returns nil and the error.
func New(name string, options ...func(e *Environment)) (*Environment, error) {
	e := &Environment{
		Name:    name,
		profile: "default",
	}
	for _, opt := range options {
		opt(e)
	}

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile:           e.profile,
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}

	client := sts.New(sess)
	resp, err := client.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}

	parsed, err := arn.Parse(*resp.Arn)
	if err != nil {
		return nil, err
	}
	e.AccountID = parsed.AccountID
	e.Region = *sess.Config.Region
	return e, nil
}

// WithProfile returns a function that can be used to override the AWS profile for creating the environment.
func WithProfile(profile string) func(e *Environment) {
	return func(e *Environment) {
		e.profile = profile
	}
}

// Marshal serializes the environment into a JSON document and returns it.
// If an error occurred during the serialization, the empty string and the error is returned.
func (e *Environment) Marshal() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
