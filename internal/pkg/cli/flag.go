// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

// Long flag names.
const (
	// Common flags.
	nameFlag    = "name"
	appFlag     = "app"
	envFlag     = "env"
	svcFlag     = "svc"
	svcTypeFlag = "svc-type"
	jobTypeFlag = "job-type"
	profileFlag = "profile"
	yesFlag     = "yes"
	jsonFlag    = "json"
	allFlag     = "all"

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
	tasksFlag             = "tasks"
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
	executionRoleFlag  = "execution-role"
	subnetsFlag        = "subnets"
	securityGroupsFlag = "security-groups"
	envVarsFlag        = "env-vars"
	commandFlag        = "command"
	taskDefaultFlag    = "default"

	vpcIDFlag          = "import-vpc-id"
	publicSubnetsFlag  = "import-public-subnets"
	privateSubnetsFlag = "import-private-subnets"

	vpcCIDRFlag            = "override-vpc-cidr"
	publicSubnetCIDRsFlag  = "override-public-cidrs"
	privateSubnetCIDRsFlag = "override-private-cidrs"

	defaultConfigFlag = "default-config"

	accessKeyIDFlag     = "aws-access-key-id"
	secretAccessKeyFlag = "aws-secret-access-key"
	sessionTokenFlag    = "aws-session-token"
	regionFlag          = "region"

	retriesFlag  = "retries"
	timeoutFlag  = "timeout"
	scheduleFlag = "schedule"
)

// Short flag names.
// A short flag only exists if the flag is mandatory by the command.
const (
	nameFlagShort    = "n"
	appFlagShort     = "a"
	envFlagShort     = "e"
	svcFlagShort     = "s"
	svcTypeFlagShort = "t"
	jobTypeFlagShort = "t"

	dockerFileFlagShort        = "d"
	githubURLFlagShort         = "u"
	githubAccessTokenFlagShort = "t"
	gitBranchFlagShort         = "b"
	envsFlagShort              = "e"

	scheduleFlagShort = "s"
)

// Descriptions for flags.
var (
	svcTypeFlagDescription = fmt.Sprintf(`Type of service to create. Must be one of:
%s`, strings.Join(template.QuoteSliceFunc(manifest.ServiceTypes), ", "))
	storageTypeFlagDescription = fmt.Sprintf(`Type of storage to add. Must be one of:
%s`, strings.Join(template.QuoteSliceFunc(storageTypes), ", "))
	jobTypeFlagDescription = fmt.Sprintf(`Type of job to create. Must be one of:
%s`, strings.Join(template.QuoteSliceFunc(manifest.JobTypes), ", "))

	subnetsFlagDescription = fmt.Sprintf(`Optional. The subnet IDs for the task to use. Can be specified multiple times.
Cannot be specified with '%s', '%s' or '%s'.`, appFlag, envFlag, taskDefaultFlag)
	securityGroupsFlagDescription = fmt.Sprintf(`Optional. The security group IDs for the task to use. Can be specified multiple times.
Cannot be specified with '%s' or '%s'.`, appFlag, envFlag)
	taskDefaultFlagDescription = fmt.Sprintf(`Optional. Run tasks in default cluster and default subnets. 
Cannot be specified with '%s', '%s' or '%s'.`, appFlag, envFlag, subnetsFlag)
	taskEnvFlagDescription = fmt.Sprintf(`Optional. Name of the environment.
Cannot be specified with '%s', '%s' or '%s'`, taskDefaultFlag, subnetsFlag, securityGroupsFlag)
	taskAppFlagDescription = fmt.Sprintf(`Optional. Name of the application.
Cannot be specified with '%s', '%s' or '%s'`, taskDefaultFlag, subnetsFlag, securityGroupsFlag)
)

const (
	appFlagDescription      = "Name of the application."
	envFlagDescription      = "Name of the environment."
	svcFlagDescription      = "Name of the service."
	jobFlagDescription      = "Name of the scheduled job."
	pipelineFlagDescription = "Name of the pipeline."
	profileFlagDescription  = "Name of the profile."
	yesFlagDescription      = "Skips confirmation prompt."
	jsonFlagDescription     = "Optional. Outputs in JSON format."

	dockerFileFlagDescription   = "Path to the Dockerfile."
	imageTagFlagDescription     = `Optional. The container image tag.`
	resourceTagsFlagDescription = `Optional. Labels with a key and value separated with commas.
Allows you to categorize resources.`
	stackOutputDirFlagDescription = "Optional. Writes the stack template and template configuration to a directory."
	prodEnvFlagDescription        = "If the environment contains production services."

	limitFlagDescription = `Optional. The maximum number of log events returned. Default is 10
unless any time filtering flags are set.`
	followFlagDescription = "Optional. Specifies if the logs should be streamed."
	sinceFlagDescription  = `Optional. Only return logs newer than a relative duration like 5s, 2m, or 3h.
Defaults to all logs. Only one of start-time / since may be used.`
	startTimeFlagDescription = `Optional. Only return logs after a specific date (RFC3339).
Defaults to all logs. Only one of start-time / since may be used.`
	endTimeFlagDescription = `Optional. Only return logs before a specific date (RFC3339).
Defaults to all logs. Only one of end-time / follow may be used.`
	tasksLogsFlagDescription = "Optional. Only return logs from specific task IDs."

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
	storageNoLSIFlagDescription     = `Optional. Don't ask about configuring alternate sort keys.`
	storageLSIConfigFlagDescription = `Optional. Attribute to use as an alternate sort key. May be specified up to 5 times.
Must be of the format '<keyName>:<dataType>'.`

	countFlagDescription         = "Optional. The number of tasks to set up."
	cpuFlagDescription           = "Optional. The number of CPU units to reserve for each task."
	memoryFlagDescription        = "Optional. The amount of memory to reserve in MiB for each task."
	imageFlagDescription         = "Optional. The image to run instead of building a Dockerfile."
	taskRoleFlagDescription      = "Optional. The ARN of the role for the task to use."
	executionRoleFlagDescription = "Optional. The ARN of the role that grants the container agent permission to make AWS API calls."
	envVarsFlagDescription       = "Optional. Environment variables specified by key=value separated with commas."
	commandFlagDescription       = `Optional. The command that is passed to "docker run" to override the default command.`
	taskGroupFlagDescription     = `Optional. The group name of the task. 
Tasks with the same group name share the same set of resources. 
(default directory name)`
	taskImageTagFlagDescription = `Optional. The container image tag in addition to "latest".`

	vpcIDFlagDescription          = "Optional. Use an existing VPC ID."
	publicSubnetsFlagDescription  = "Optional. Use existing public subnet IDs."
	privateSubnetsFlagDescription = "Optional. Use existing private subnet IDs."

	vpcCIDRFlagDescription            = "Optional. Global CIDR to use for VPC (default 10.0.0.0/16)."
	publicSubnetCIDRsFlagDescription  = "Optional. CIDR to use for public subnets (default 10.0.0.0/24,10.0.1.0/24)."
	privateSubnetCIDRsFlagDescription = "Optional. CIDR to use for private subnets (default 10.0.2.0/24,10.0.3.0/24)."

	defaultConfigFlagDescription = "Optional. Skip prompting and use default environment configuration."

	accessKeyIDFlagDescription     = "Optional. An AWS access key."
	secretAccessKeyFlagDescription = "Optional. An AWS secret access key."
	sessionTokenFlagDescription    = "Optional. An AWS session token for temporary credentials."
	envRegionTokenFlagDescription  = "Optional. An AWS region where the environment will be created."

	retriesFlagDescription = "Optional. The number of times to try restarting the job on a failure."
	timeoutFlagDescription = `Optional. The total execution time for the task, including retries.
Accepts valid Go duration strings. For example: "2h", "1h30m", "900s".`
	scheduleFlagDescription = `The schedule on which to run this job. 
Accepts cron expressions of the format (M H DoM M DoW) and schedule definition strings. 
For example: "0 * * * *", "@daily", "@weekly", "@every 1h30m".`

	upgradeAllEnvsDescription = "Optional. Upgrade all environments."
)
