// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
)

// Long flag names.
const (
	// Common flags.
	nameFlag    = "name"
	appFlag     = "app"
	envFlag     = "env"
	svcFlag     = "svc"
	svcTypeFlag = "svc-type"
	profileFlag = "profile"
	yesFlag     = "yes"
	jsonFlag    = "json"

	// Command specific flags.
	dockerFileFlag        = "dockerfile"
	imageTagFlag          = "tag"
	resourceTagsFlag      = "resource-tags"
	stackOutputDirFlag    = "output-dir"
	limitFlag             = "limit"
	followFlag            = "follow"
	sinceFlag             = "since"
	startTimeFlag         = "start-time"
	endTimeFlag           = "end-time"
	envProfilesFlag       = "env-profiles"
	prodEnvFlag           = "prod"
	deployFlag            = "deploy"
	resourcesFlag         = "resources"
	githubURLFlag         = "github-url"
	githubAccessTokenFlag = "github-access-token"
	gitBranchFlag         = "git-branch"
	envsFlag              = "environments"
	domainNameFlag        = "domain"
	localFlag             = "local"
	deleteSecretFlag      = "delete-secret"
	svcPortFlag           = "port"

	storageTypeFlag         = "storage-type"
	storagePartitionKeyFlag = "partition-key"
	storageSortKeyFlag      = "sort-key"
	storageNoSortFlag       = "no-sort"
	storageLSIConfigFlag    = "lsi"
	storageNoLSIFlag        = "no-lsi"

	taskGroupNameFlag  = "task-group-name"
	countFlag          = "count"
	cpuFlag            = "cpu"
	memoryFlag         = "memory"
	imageFlag          = "image"
	taskRoleFlag       = "task-role"
	subnetFlag         = "subnet"
	securityGroupsFlag = "security-groups"
	envVarsFlag        = "env-vars"
	commandsFlag       = "commands"
)

// Short flag names.
// A short flag only exists if the flag is mandatory by the command.
const (
	nameFlagShort    = "n"
	appFlagShort     = "a"
	envFlagShort     = "e"
	svcFlagShort     = "s"
	svcTypeFlagShort = "t"

	dockerFileFlagShort        = "d"
	githubURLFlagShort         = "u"
	githubAccessTokenFlagShort = "t"
	gitBranchFlagShort         = "b"
	envsFlagShort              = "e"
)

// Descriptions for flags.
var (
	svcTypeFlagDescription = fmt.Sprintf(`Type of service to create. Must be one of:
%s`, strings.Join(template.QuoteSliceFunc(manifest.ServiceTypes), ", "))
	storageTypeFlagDescription = fmt.Sprintf(`Type of storage to add. Must be one of:
%s`, strings.Join(template.QuoteSliceFunc(storageTypes), ", "))
)

const (
	appFlagDescription      = "Name of the application."
	envFlagDescription      = "Name of the environment."
	svcFlagDescription      = "Name of the service."
	pipelineFlagDescription = "Name of the pipeline."
	profileFlagDescription  = "Name of the profile."
	yesFlagDescription      = "Skips confirmation prompt."
	jsonFlagDescription     = "Optional. Outputs in JSON format."

	dockerFileFlagDescription   = "Path to the Dockerfile."
	imageTagFlagDescription     = `Optional. The service's image tag.`
	resourceTagsFlagDescription = `Optional. Labels with a key and value separated with commas.
Allows you to categorize resources.`
	stackOutputDirFlagDescription = "Optional. Writes the stack template and template configuration to a directory."
	prodEnvFlagDescription        = "If the environment contains production services."
	limitFlagDescription          = "Optional. The maximum number of log events returned."
	followFlagDescription         = "Optional. Specifies if the logs should be streamed."
	sinceFlagDescription          = `Optional. Only return logs newer than a relative duration like 5s, 2m, or 3h.
Defaults to all logs. Only one of start-time / since may be used.`
	startTimeFlagDescription = `Optional. Only return logs after a specific date (RFC3339).
Defaults to all logs. Only one of start-time / since may be used.`
	endTimeFlagDescription = `Optional. Only return logs before a specific date (RFC3339).
Defaults to all logs. Only one of end-time / follow may be used.`
	deployTestFlagDescription        = `Deploy your service to a "test" environment.`
	githubURLFlagDescription         = "GitHub repository URL for your service."
	githubAccessTokenFlagDescription = "GitHub personal access token for your repository."
	gitBranchFlagDescription         = "Branch used to trigger your pipeline."
	pipelineEnvsFlagDescription      = "Environments to add to the pipeline."
	domainNameFlagDescription        = "Optional. Your existing custom domain name."
	envResourcesFlagDescription      = "Optional. Show the resources in your environment."
	svcResourcesFlagDescription      = "Optional. Show the resources in your service."
	pipelineResourcesFlagDescription = "Optional. Show the resources in your pipeline."
	localSvcFlagDescription          = "Only show services in the workspace."
	envProfilesFlagDescription       = "Optional. Environments and the profile to use to delete the environment."
	deleteSecretFlagDescription      = "Deletes AWS Secrets Manager secret associated with a pipeline source repository."
	svcPortFlagDescription           = "Optional. The port on which your service listens."

	storageFlagDescription             = "Name of the storage resource to create."
	storageServiceFlagDescription      = "Name of the service to associate with storage."
	storagePartitionKeyFlagDescription = `Partition key for the DDB table.
Must be of the format '<keyName>:<dataType>'.`
	storageSortKeyFlagDescription = `Optional. Sort key for the DDB table.
Must be of the format '<keyName>:<dataType>'.`
	storageNoSortFlagDescription    = "Optional. Skip configuring sort keys."
	storageNoLsiFlagDescription     = `Optional. Don't ask about configuring alternate sort keys.`
	storageLSIConfigFlagDescription = `Optional. Attribute to use as an alternate sort key. May be specified up to 5 times.
Must be of the format '<keyName>:<dataType>'.`

	countFlagDescription          = "Optional. The number of tasks to set up. Default 1."
	cpuFlagDescription            = "Optional. The number of CPU units to reserve for each task. Default 256 (1/4 vCPU)."
	memoryFlagDescription         = "Optional. The amount of memory to reserve in MiB for each task. Default 512."
	imageFlagDescription          = "Optional. The image to run instead of building a Dockerfile."
	taskRoleFlagDescription       = "Optional. The role for the task to use."
	subnetFlagDescription         = "Optional. The subnet id for the task to use."
	securityGroupsFlagDescription = "Optional. The security group id(s) for the task to use. Can be specified multiple times."
	envVarsFlagDescription        = "Optional. Environment variables specified by key=value separated with commas."
	commandsFlagDescription       = "Optional. List of commands that are passed to docker run. Can be specified multiple times."
	taskGroupFlagDescription      = "The group name of the task. Tasks with the same group name share the same set of resources."
)
