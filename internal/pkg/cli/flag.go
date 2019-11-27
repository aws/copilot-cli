// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

// Long flag names.
const (
	// Common flags.
	projectFlag = "project"
	nameFlag    = "name"
	appFlag     = "app"
	envFlag     = "env"
	appTypeFlag = "app-type"
	profileFlag = "profile"
	yesFlag     = "yes"
	jsonFlag    = "json"

	// Command specific flags.
	dockerFileFlag        = "dockerfile"
	imageTagFlag          = "tag"
	stackOutputDirFlag    = "output-dir"
	prodEnvFlag           = "prod"
	deployFlag            = "deploy"
	githubRepoFlag        = "github-repo"
	githubAccessTokenFlag = "github-access-token"
	enableCDFlag          = "enable-cd"
	envsFlag              = "environments"
	domainNameFlag        = "domain"
	pipelineFileFlag      = "file"
	appLocalFlag          = "local"
)

// Short flag names.
// A short flag only exists if the flag is mandatory by the command.
const (
	projectFlagShort = "p"
	nameFlagShort    = "n"
	appFlagShort     = "a"
	envFlagShort     = "e"
	appTypeFlagShort = "t"

	dockerFileFlagShort        = "d"
	githubRepoFlagShort        = "r"
	githubAccessTokenFlagShort = "t"
	envsFlagShort              = "e"
	pipelineFileFlagShort      = "f"
)

// Descriptions for flags.
const (
	projectFlagDescription = "Name of the project."
	appFlagDescription     = "Name of the application."
	envFlagDescription     = "Name of the environment."
	appTypeFlagDescription = "Type of application to create."
	profileFlagDescription = "Name of the profile."
	yesFlagDescription     = "Skips confirmation prompt."
	jsonFlagDescription    = "Output in JSON format."

	dockerFileFlagDescription        = "Path to the Dockerfile."
	imageTagFlagDescription          = `Optional. The application's image tag.`
	stackOutputDirFlagDescription    = "Optional. Writes the stack template and template configuration to a directory."
	prodEnvFlagDescription           = "If the environment contains production services."
	deployTestFlagDescription        = `Deploy your application to a "test" environment.`
	githubRepoFlagDescription        = "GitHub repository for your application."
	githubAccessTokenFlagDescription = "GitHub personal access token for your repository."
	deployPipelineFlagDescription    = "Deploys the pipeline."
	enableCDFlagDescription          = "Enables automatic deployment to production environment."
	pipelineEnvsFlagDescription      = "Environments to add to the pipeline."
	domainNameFlagDescription        = "Optional. Your existing custom domain name."
	pipelineFileFlagDescription      = "Name of YAML file used to update the pipeline."
	deployFlagDescription            = "Trigger a deployment of your application(s) to any new stage in your pipeline."
	appLocalFlagDescription          = "Only show applications in the current directory."
)
