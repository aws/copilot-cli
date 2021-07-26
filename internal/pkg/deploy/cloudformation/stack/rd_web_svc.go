// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

type requestDrivenWebSvcReadParser interface {
	template.ReadParser
	ParseRequestDrivenWebService(template.ParseRequestDrivenWebServiceInput) (*template.Content, error)
}

// RequestDrivenWebService represents the configuration needed to create a CloudFormation stack from a request-drive web service manifest.
type RequestDrivenWebService struct {
	*appRunnerWkld
	manifest            *manifest.RequestDrivenWebService
	app                 deploy.AppInformation
	customResourceS3URL map[string]string

	parser requestDrivenWebSvcReadParser
}

// NewRequestDrivenWebServiceWithAlias creates a new RequestDrivenWebService stack from a manifest file. It creates
// custom resources needed for alias with scripts accessible from the urls.
func NewRequestDrivenWebServiceWithAlias(mft *manifest.RequestDrivenWebService, env string, app deploy.AppInformation, rc RuntimeConfig, urls map[string]string) (*RequestDrivenWebService, error) {
	rdSvc, err := NewRequestDrivenWebService(mft, env, app, rc)
	if err != nil {
		return nil, err
	}
	rdSvc.customResourceS3URL = urls
	return rdSvc, nil
}

// NewRequestDrivenWebService creates a new RequestDrivenWebService stack from a manifest file.
func NewRequestDrivenWebService(mft *manifest.RequestDrivenWebService, env string, app deploy.AppInformation, rc RuntimeConfig) (*RequestDrivenWebService, error) {
	parser := template.New()
	addons, err := addon.New(aws.StringValue(mft.Name))
	if err != nil {
		return nil, fmt.Errorf("new addons: %w", err)
	}
	return &RequestDrivenWebService{
		appRunnerWkld: &appRunnerWkld{
			wkld: &wkld{
				name:   aws.StringValue(mft.Name),
				env:    env,
				app:    app.Name,
				rc:     rc,
				image:  mft.ImageConfig,
				addons: addons,
				parser: parser,
			},
			instanceConfig:    mft.InstanceConfig,
			imageConfig:       mft.ImageConfig,
			healthCheckConfig: mft.HealthCheckConfiguration,
		},
		app:      app,
		manifest: mft,
		parser:   parser,
	}, nil
}

// Template returns the CloudFormation template for the service parametrized for the environment.
func (s *RequestDrivenWebService) Template() (string, error) {
	outputs, err := s.addonsOutputs()
	if err != nil {
		return "", err
	}

	bucket, urls, err := parseS3URLs(s.customResourceS3URL)
	if err != nil {
		return "", err
	}

	dnsDelegationRole, dnsName := convertAppInformation(s.app)

	publishers, err := convertPublish(s.manifest.Publish, s.rc.AccountID, s.rc.Region, s.app.Name, s.env, s.name)
	if err != nil {
		return "", fmt.Errorf(`convert "publish" field for service %s: %w`, s.name, err)
	}

	content, err := s.parser.ParseRequestDrivenWebService(template.ParseRequestDrivenWebServiceInput{
		Variables:         s.manifest.Variables,
		Tags:              s.manifest.Tags,
		NestedStack:       outputs,
		EnableHealthCheck: !s.healthCheckConfig.IsEmpty(),

		Alias:                s.manifest.Alias,
		ScriptBucketName:     bucket,
		CustomDomainLambda:   urls[template.AppRunnerCustomDomainLambdaFileName],
		AWSSDKLayer:          urls[template.AWSSDKLayerFileName],
		AppDNSDelegationRole: dnsDelegationRole,
		AppDNSName:           dnsName,

		Publish: publishers,
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (s *RequestDrivenWebService) SerializedParameters() (string, error) {
	return s.templateConfiguration(s)
}
