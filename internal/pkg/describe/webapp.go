// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/aws-sdk-go/aws"
	clientSession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// WebAppURI represents the unique identifier to access a web application.
type WebAppURI struct {
	DNSName string // The environment's subdomain if the application is served on HTTPS. Otherwise, the public load balancer's DNS.
	Path    string // Empty if the application is served on HTTPS. Otherwise, the pattern used to match the application.
}

type stackDescriber interface {
	DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
}

type sessionFromRoleProvider interface {
	FromRole(roleARN string, region string) (*clientSession.Session, error)
}

type envGetter interface {
	archer.EnvironmentGetter
}

func (uri *WebAppURI) String() string {
	if uri.Path != "" {
		return fmt.Sprintf("%s and path %s", color.HighlightResource("http://"+uri.DNSName), color.HighlightResource(uri.Path))
	}
	return color.HighlightResource("https://" + uri.DNSName)
}

// WebAppDescriber retrieves information about a load balanced web application.
type WebAppDescriber struct {
	app *archer.Application

	store           envGetter
	stackDescribers map[string]stackDescriber
	sessProvider    sessionFromRoleProvider
}

// NewWebAppDescriber instantiates a load balanced application.
func NewWebAppDescriber(project, app string) (*WebAppDescriber, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := svc.GetApplication(project, app)
	if err != nil {
		return nil, err
	}
	return &WebAppDescriber{
		app:             meta,
		store:           svc,
		stackDescribers: make(map[string]stackDescriber),
		sessProvider:    session.NewProvider(),
	}, nil
}

// URI returns the stringified WebAppURI to identify this application uniquely given an environment name.
func (d *WebAppDescriber) URI(envName string) (*WebAppURI, error) {
	env, err := d.store.GetEnvironment(d.app.Project, envName)
	if err != nil {
		return nil, err
	}

	envOutputs, err := d.envOutputs(env)
	if err != nil {
		return nil, err
	}
	appParams, err := d.appParams(env)
	if err != nil {
		return nil, err
	}

	uri := &WebAppURI{
		DNSName: envOutputs[stack.EnvOutputPublicLoadBalancerDNSName],
		Path:    appParams[stack.LBFargateRulePathKey],
	}
	_, isHTTPS := envOutputs[stack.EnvOutputSubdomain]
	if isHTTPS {
		dnsName := fmt.Sprintf("%s.%s", d.app.Name, envOutputs[stack.EnvOutputSubdomain])
		uri = &WebAppURI{
			DNSName: dnsName,
		}
	}
	return uri, nil
}

func (d *WebAppDescriber) envOutputs(env *archer.Environment) (map[string]string, error) {
	envStack, err := d.stack(env.ManagerRoleARN, env.Region, stack.NameForEnv(d.app.Project, env.Name))
	if err != nil {
		return nil, err
	}
	outputs := make(map[string]string)
	for _, out := range envStack.Outputs {
		outputs[*out.OutputKey] = *out.OutputValue
	}
	return outputs, nil
}

func (d *WebAppDescriber) appParams(env *archer.Environment) (map[string]string, error) {
	appStack, err := d.stack(env.ManagerRoleARN, env.Region, stack.NameForApp(d.app.Project, env.Name, d.app.Name))
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	for _, param := range appStack.Parameters {
		params[*param.ParameterKey] = *param.ParameterValue
	}
	return params, nil
}

func (d *WebAppDescriber) stack(roleARN, region, stackName string) (*cloudformation.Stack, error) {
	svc, err := d.stackDescriber(roleARN, region)
	if err != nil {
		return nil, err
	}
	out, err := svc.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe stack %s: %w", stackName, err)
	}
	if len(out.Stacks) == 0 {
		return nil, fmt.Errorf("stack %s not found", stackName)
	}
	return out.Stacks[0], nil
}

func (d *WebAppDescriber) stackDescriber(roleARN, region string) (stackDescriber, error) {
	if _, ok := d.stackDescribers[roleARN]; !ok {
		sess, err := d.sessProvider.FromRole(roleARN, region)
		if err != nil {
			return nil, fmt.Errorf("session for role %s and region %s: %w", roleARN, region, err)
		}
		d.stackDescribers[roleARN] = cloudformation.New(sess)
	}
	return d.stackDescribers[roleARN], nil
}
