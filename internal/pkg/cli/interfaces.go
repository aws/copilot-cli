// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecr"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/aws/aws-sdk-go/aws/session"
)

// actionCommand is the interface that every command that creates a resource implements.
type actionCommand interface {
	// Validate returns an error if a flag's value is invalid.
	Validate() error

	// Ask prompts for flag values that are required but not passed in.
	Ask() error

	// Execute runs the command after collecting all required options.
	Execute() error

	// RecommendedActions returns a list of follow-up suggestions users can run once the command executes successfully.
	RecommendedActions() []string
}

type projectService interface {
	archer.ProjectStore
	archer.EnvironmentStore
	archer.ApplicationStore
}

type ecrService interface {
	GetRepository(name string) (string, error)
	GetECRAuth() (ecr.Auth, error)
}

type cwlogService interface {
	TaskLogEvents(logGroupName string, stringTokens map[string]*string, opts ...cloudwatchlogs.GetLogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
	LogGroupExists(logGroupName string) (bool, error)
}

type templater interface {
	Template() (string, error)
}

type dockerService interface {
	Build(uri, tag, path string) error
	Login(uri, username, password string) error
	Push(uri, tag string) error
}

type runner interface {
	Run(name string, args []string, options ...command.Option) error
}

type defaultSessionProvider interface {
	Default() (*session.Session, error)
}

type regionalSessionProvider interface {
	DefaultWithRegion(region string) (*session.Session, error)
}

type sessionFromRoleProvider interface {
	FromRole(roleARN string, region string) (*session.Session, error)
}

type profileNames interface {
	Names() []string
}

type sessionProvider interface {
	defaultSessionProvider
	regionalSessionProvider
	sessionFromRoleProvider
}

type webAppDescriber interface {
	URI(envName string) (*describe.WebAppURI, error)
	ECSParams(envName string) (*describe.WebAppECSParams, error)
	EnvVars(env *archer.Environment) ([]*describe.WebAppEnvVars, error)
	StackResources(envName string) ([]*describe.CfnResource, error)
}

type storeReader interface {
	archer.ProjectLister
	archer.ProjectGetter
	archer.EnvironmentLister
	archer.EnvironmentGetter
	archer.ApplicationLister
	archer.ApplicationGetter
}

type wsAppManifestReader interface {
	ReadAppManifest(appName string) ([]byte, error)
}

type wsAppManifestWriter interface {
	WriteAppManifest(marshaler encoding.BinaryMarshaler, appName string) (string, error)
}

type wsPipelineManifestReader interface {
	ReadPipelineManifest() ([]byte, error)
}

type wsPipelineWriter interface {
	WritePipelineBuildspec(marshaler encoding.BinaryMarshaler) (string, error)
	WritePipelineManifest(marshaler encoding.BinaryMarshaler) (string, error)
}

type wsAppDeleter interface {
	DeleteApp(name string) error
}

type wsAppReader interface {
	AppNames() ([]string, error)
	wsAppManifestReader
}

type wsPipelineDeleter interface {
	DeletePipelineManifest() error
	wsPipelineManifestReader
}

type wsPipelineReader interface {
	AppNames() ([]string, error)
	wsPipelineManifestReader
}

type wsProjectManager interface {
	Create(projectName string) error
	Summary() (*workspace.Summary, error)
}
