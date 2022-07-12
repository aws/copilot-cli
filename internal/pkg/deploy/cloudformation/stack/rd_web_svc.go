// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

var awsSDKLayerForRegion = map[string]*string{
	"ap-northeast-1": aws.String("arn:aws:lambda:ap-northeast-1:249908578461:layer:AWSLambda-Node-AWS-SDK:15"),
	"us-east-1":      aws.String("arn:aws:lambda:us-east-1:668099181075:layer:AWSLambda-Node-AWS-SDK:15"),
	"ap-southeast-1": aws.String("arn:aws:lambda:ap-southeast-1:468957933125:layer:AWSLambda-Node-AWS-SDK:14"),
	"eu-west-1":      aws.String("arn:aws:lambda:eu-west-1:399891621064:layer:AWSLambda-Node-AWS-SDK:14"),
	"us-west-1":      aws.String("arn:aws:lambda:us-west-1:325793726646:layer:AWSLambda-Node-AWS-SDK:15"),
	"ap-northeast-2": aws.String("arn:aws:lambda:ap-northeast-2:296580773974:layer:AWSLambda-Node-AWS-SDK:14"),
	"ap-south-1":     aws.String("arn:aws:lambda:ap-south-1:631267018583:layer:AWSLambda-Node-AWS-SDK:14"),
	"ap-southeast-2": aws.String("arn:aws:lambda:ap-southeast-2:817496625479:layer:AWSLambda-Node-AWS-SDK:14"),
	"ca-central-1":   aws.String("arn:aws:lambda:ca-central-1:778625758767:layer:AWSLambda-Node-AWS-SDK:14"),
	"eu-central-1":   aws.String("arn:aws:lambda:eu-central-1:292169987271:layer:AWSLambda-Node-AWS-SDK:14"),
	"eu-west-2":      aws.String("arn:aws:lambda:eu-west-2:142628438157:layer:AWSLambda-Node-AWS-SDK:14"),
	"sa-east-1":      aws.String("arn:aws:lambda:sa-east-1:640010853179:layer:AWSLambda-Node-AWS-SDK:14"),
	"us-east-2":      aws.String("arn:aws:lambda:us-east-2:259788987135:layer:AWSLambda-Node-AWS-SDK:14"),
	"us-west-2":      aws.String("arn:aws:lambda:us-west-2:420165488524:layer:AWSLambda-Node-AWS-SDK:14"),
	"af-south-1":     aws.String("arn:aws:lambda:af-south-1:392341123054:layer:AWSLambda-Node-AWS-SDK:7"),
	"ap-east-1":      aws.String("arn:aws:lambda:ap-east-1:118857876118:layer:AWSLambda-Node-AWS-SDK:14"),
	"ap-northeast-3": aws.String("arn:aws:lambda:ap-northeast-3:961244031340:layer:AWSLambda-Node-AWS-SDK:14"),
	"eu-north-1":     aws.String("arn:aws:lambda:eu-north-1:642425348156:layer:AWSLambda-Node-AWS-SDK:14"),
	"eu-south-1":     aws.String("arn:aws:lambda:eu-south-1:426215560912:layer:AWSLambda-Node-AWS-SDK:7"),
	"eu-west-3":      aws.String("arn:aws:lambda:eu-west-3:959311844005:layer:AWSLambda-Node-AWS-SDK:14"),
	"me-south-1":     aws.String("arn:aws:lambda:me-south-1:507411403535:layer:AWSLambda-Node-AWS-SDK:10"),
}

type requestDrivenWebSvcReadParser interface {
	template.ReadParser
	ParseRequestDrivenWebService(template.WorkloadOpts) (*template.Content, error)
}

// RequestDrivenWebService represents the configuration needed to create a CloudFormation stack from a request-drive web service manifest.
type RequestDrivenWebService struct {
	*appRunnerWkld
	manifest *manifest.RequestDrivenWebService
	app      deploy.AppInformation

	parser requestDrivenWebSvcReadParser
}

type RequestDrivenWebServiceConfig struct {
	App           deploy.AppInformation
	EnvName       string
	Manifest      *manifest.RequestDrivenWebService
	RuntimeConfig RuntimeConfig
	Addons        addons
}

// NewRequestDrivenWebService creates a new RequestDrivenWebService stack from a manifest file.
func NewRequestDrivenWebService(conf RequestDrivenWebServiceConfig) (*RequestDrivenWebService, error) {
	parser := template.New()
	return &RequestDrivenWebService{
		appRunnerWkld: &appRunnerWkld{
			wkld: &wkld{
				name:   aws.StringValue(conf.Manifest.Name),
				env:    conf.EnvName,
				app:    conf.App.Name,
				rc:     conf.RuntimeConfig,
				image:  conf.Manifest.ImageConfig.Image,
				addons: conf.Addons,
				parser: parser,
			},
			instanceConfig:    conf.Manifest.InstanceConfig,
			imageConfig:       conf.Manifest.ImageConfig,
			healthCheckConfig: conf.Manifest.HealthCheckConfiguration,
		},
		app:      conf.App,
		manifest: conf.Manifest,
		parser:   parser,
	}, nil
}

// Template returns the CloudFormation template for the service parametrized for the environment.
func (s *RequestDrivenWebService) Template() (string, error) {
	crs, err := convertCustomResources(s.rc.CustomResourcesURL)
	if err != nil {
		return "", err
	}
	networkConfig := convertRDWSNetworkConfig(s.manifest.Network)
	addonsParams, err := s.addonsParameters()
	if err != nil {
		return "", err
	}
	addonsOutputs, err := s.addonsOutputs()
	if err != nil {
		return "", err
	}
	var layerARN, dnsDelegationRole, dnsName *string
	if s.manifest.Alias != nil {
		dnsDelegationRole, dnsName = convertAppInformation(s.app)
		layerARN = awsSDKLayerForRegion[s.rc.Region]
	}
	publishers, err := convertPublish(s.manifest.Publish(), s.rc.AccountID, s.rc.Region, s.app.Name, s.env, s.name)
	if err != nil {
		return "", fmt.Errorf(`convert "publish" field for service %s: %w`, s.name, err)
	}
	content, err := s.parser.ParseRequestDrivenWebService(template.WorkloadOpts{
		AppName:              s.wkld.app,
		EnvName:              s.env,
		WorkloadName:         s.name,
		Variables:            s.manifest.Variables,
		StartCommand:         s.manifest.StartCommand,
		Tags:                 s.manifest.Tags,
		NestedStack:          addonsOutputs,
		AddonsExtraParams:    addonsParams,
		EnableHealthCheck:    !s.healthCheckConfig.IsEmpty(),
		WorkloadType:         manifest.RequestDrivenWebServiceType,
		Alias:                s.manifest.Alias,
		CustomResources:      crs,
		AWSSDKLayer:          layerARN,
		AppDNSDelegationRole: dnsDelegationRole,
		AppDNSName:           dnsName,
		Network:              networkConfig,

		Publish:                  publishers,
		ServiceDiscoveryEndpoint: s.rc.ServiceDiscoveryEndpoint,

		Observability: template.ObservabilityOpts{
			Tracing: strings.ToUpper(aws.StringValue(s.manifest.Observability.Tracing)),
		},
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
