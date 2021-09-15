// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package sessions provides functions that return AWS sessions to use in the AWS SDK.
package sessions

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/copilot-cli/internal/pkg/version"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	userAgentHeader = "User-Agent"

	maxRetriesOnRecoverableFailures = 8 // Default provided by SDK is 3 which means requests are retried up to only 2 seconds.
	credsTimeout                    = 10 * time.Second
	clientTimeout                   = 30 * time.Second
)

// Provider provides methods to create sessions.
// Once a session is created, it's cached locally so that the same session is not re-created.
type Provider struct {
	defaultSess *session.Session
}

var instance *Provider
var once sync.Once

// NewProvider returns a session Provider singleton.
func NewProvider() *Provider {
	once.Do(func() {
		instance = &Provider{}
	})
	return instance
}

// Default returns a session configured against the "default" AWS profile.
func (p *Provider) Default() (*session.Session, error) {
	if p.defaultSess != nil {
		return p.defaultSess, nil
	}

	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            *newConfig(),
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}
	if aws.StringValue(sess.Config.Region) == "" {
		return nil, &errMissingRegion{}
	}

	sess.Handlers.Build.PushBackNamed(userAgentHandler())
	p.defaultSess = sess
	return sess, nil
}

// DefaultWithRegion returns a session configured against the "default" AWS profile and the input region.
func (p *Provider) DefaultWithRegion(region string) (*session.Session, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            *newConfig().WithRegion(region),
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}
	sess.Handlers.Build.PushBackNamed(userAgentHandler())
	return sess, nil
}

// FromProfile returns a session configured against the input profile name.
func (p *Provider) FromProfile(name string) (*session.Session, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            *newConfig(),
		SharedConfigState: session.SharedConfigEnable,
		Profile:           name,
	})
	if err != nil {
		return nil, err
	}
	if aws.StringValue(sess.Config.Region) == "" {
		return nil, &errMissingRegion{}
	}
	sess.Handlers.Build.PushBackNamed(userAgentHandler())
	return sess, nil
}

// FromRole returns a session configured against the input role and region.
func (p *Provider) FromRole(roleARN string, region string) (*session.Session, error) {
	defaultSession, err := session.NewSessionWithOptions(session.Options{
		Config:            *newConfig(),
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating default session: %w", err)
	}
	defaultSession.Handlers.Build.PushBackNamed(userAgentHandler())

	creds := stscreds.NewCredentials(defaultSession, roleARN)
	sess, err := session.NewSession(
		newConfig().
			WithCredentials(creds).
			WithRegion(region),
	)
	if err != nil {
		return nil, err
	}
	sess.Handlers.Build.PushBackNamed(userAgentHandler())
	return sess, nil
}

// FromStaticCreds returns a session from static credentials.
func (p *Provider) FromStaticCreds(accessKeyID, secretAccessKey, sessionToken string) (*session.Session, error) {
	conf := newConfig()
	conf.Credentials = credentials.NewStaticCredentials(accessKeyID, secretAccessKey, sessionToken)
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: *conf,
	})
	if err != nil {
		return nil, fmt.Errorf("create session from static credentials: %w", err)
	}
	sess.Handlers.Build.PushBackNamed(userAgentHandler())
	return sess, nil
}

// AreCredsFromEnvVars returns true if the session's credentials provider is environment variables, false otherwise.
// An error is returned if the credentials are invalid or the request times out.
func AreCredsFromEnvVars(sess *session.Session) (bool, error) {
	v, err := Creds(sess)
	if err != nil {
		return false, err
	}
	return v.ProviderName == session.EnvProviderName, nil
}

// Creds returns the credential values from a session.
func Creds(sess *session.Session) (credentials.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), credsTimeout)
	defer cancel()

	v, err := sess.Config.Credentials.GetWithContext(ctx)
	if err != nil {
		return credentials.Value{}, fmt.Errorf("get credentials of session: %w", err)
	}
	return v, nil
}

// newConfig returns a config with an end-to-end request timeout and verbose credentials errors.
func newConfig() *aws.Config {
	c := &http.Client{
		Timeout: clientTimeout,
	}
	return aws.NewConfig().
		WithHTTPClient(c).
		WithCredentialsChainVerboseErrors(true).
		WithMaxRetries(maxRetriesOnRecoverableFailures)
}

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
