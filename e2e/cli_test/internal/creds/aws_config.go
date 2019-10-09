// +build e2e

package creds

import (
	"fmt"
	"os"
)

type Creds struct {
	AwsAccessKey    string
	AwsSecretKey    string
	AwsSessionToken string
	AwsRegion       string
}

func ExtractAWSCredsFromEnvVars() (*Creds, error) {
	const (
		awsAccessKeyName        = "AWS_ACCESS_KEY_ID"
		awsSecretKeyName        = "AWS_SECRET_ACCESS_KEY"
		awsSessionTokenName     = "AWS_SESSION_TOKEN"
		awsDefaultRegionKeyName = "AWS_DEFAULT_REGION"
	)

	var res = &Creds{}

	value, exists := os.LookupEnv(awsAccessKeyName)
	if !exists {
		return nil, fmt.Errorf("%s is not set", awsAccessKeyName)
	}
	res.AwsAccessKey = value

	value, exists = os.LookupEnv(awsSecretKeyName)
	if !exists || value == "" {
		return nil, fmt.Errorf("%s is not set", awsSecretKeyName)
	}
	res.AwsSecretKey = value

	value, exists = os.LookupEnv(awsSessionTokenName)
	if !exists || value == "" {
		return nil, fmt.Errorf("%s is not set", awsSessionTokenName)
	}
	res.AwsSessionToken = value

	value, exists = os.LookupEnv(awsDefaultRegionKeyName)
	if !exists || value == "" {
		return nil, fmt.Errorf("%s is not set", awsDefaultRegionKeyName)
	}
	res.AwsRegion = value

	return res, nil
}
