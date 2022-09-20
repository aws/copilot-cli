// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

// Long flag names.
const (
	// Common flags.
	nameFlag       = "name"
	appFlag        = "app"
	envFlag        = "env"
	workloadFlag   = "workload"
	svcTypeFlag    = "svc-type"
	jobTypeFlag    = "job-type"
	typeFlag       = "type"
	profileFlag    = "profile"
	yesFlag        = "yes"
	jsonFlag       = "json"
	allFlag        = "all"
	forceFlag      = "force"
	noRollbackFlag = "no-rollback"
	manifestFlag   = "manifest"

	// Command specific flags.
	dockerFileFlag        = "dockerfile"
	dockerFileContextFlag = "build-context"
	imageTagFlag          = "tag"
	resourceTagsFlag      = "resource-tags"
	stackOutputDirFlag    = "output-dir"
	uploadAssetsFlag      = "upload-assets"
	limitFlag             = "limit"
	lastFlag              = "last"
	followFlag            = "follow"
	previousFlag          = "previous"
	sinceFlag             = "since"
	startTimeFlag         = "start-time"
	endTimeFlag           = "end-time"
	tasksFlag             = "tasks"
	logGroupFlag          = "log-group"
	containerLogFlag      = "container"
	prodEnvFlag           = "prod"
	deployFlag            = "deploy"
	resourcesFlag         = "resources"

	githubURLFlag         = "github-url"
	repoURLFlag           = "url"
	githubAccessTokenFlag = "github-access-token"
	gitBranchFlag         = "git-branch"
	envsFlag              = "environments"
	pipelineTypeFlag      = "pipeline-type"

	domainNameFlag   = "domain"
	localFlag        = "local"
	deleteSecretFlag = "delete-secret"
	svcPortFlag      = "port"

	noSubscriptionFlag  = "no-subscribe"
	subscribeTopicsFlag = "subscribe-topics"

	storageTypeFlag              = "storage-type"
	storagePartitionKeyFlag      = "partition-key"
	storageSortKeyFlag           = "sort-key"
	storageNoSortFlag            = "no-sort"
	storageLSIConfigFlag         = "lsi"
	storageNoLSIFlag             = "no-lsi"
	storageRDSEngineFlag         = "engine"
	storageRDSInitialDBFlag      = "initial-db"
	storageRDSParameterGroupFlag = "parameter-group"

	taskGroupNameFlag            = "task-group-name"
	countFlag                    = "count"
	cpuFlag                      = "cpu"
	memoryFlag                   = "memory"
	imageFlag                    = "image"
	taskRoleFlag                 = "task-role"
	executionRoleFlag            = "execution-role"
	clusterFlag                  = "cluster"
	acknowledgeSecretsAccessFlag = "acknowledge-secrets-access"
	subnetsFlag                  = "subnets"
	securityGroupsFlag           = "security-groups"
	envVarsFlag                  = "env-vars"
	envFileFlag                  = "env-file"
	secretsFlag                  = "secrets"
	commandFlag                  = "command"
	entrypointFlag               = "entrypoint"
	taskDefaultFlag              = "default"
	generateCommandFlag          = "generate-cmd"
	osFlag                       = "platform-os"
	archFlag                     = "platform-arch"

	vpcIDFlag                      = "import-vpc-id"
	publicSubnetsFlag              = "import-public-subnets"
	privateSubnetsFlag             = "import-private-subnets"
	certsFlag                      = "import-cert-arns"
	internalALBSubnetsFlag         = "internal-alb-subnets"
	allowVPCIngressFlag            = "internal-alb-allow-vpc-ingress"
	overrideVPCCIDRFlag            = "override-vpc-cidr"
	overrideAZsFlag                = "override-az-names"
	overridePublicSubnetCIDRsFlag  = "override-public-cidrs"
	overridePrivateSubnetCIDRsFlag = "override-private-cidrs"

	enableContainerInsightsFlag = "container-insights"

	defaultConfigFlag = "default-config"

	accessKeyIDFlag     = "aws-access-key-id"
	secretAccessKeyFlag = "aws-secret-access-key"
	sessionTokenFlag    = "aws-session-token"
	regionFlag          = "region"

	retriesFlag  = "retries"
	timeoutFlag  = "timeout"
	scheduleFlag = "schedule"

	taskIDFlag    = "task-id"
	containerFlag = "container"

	valuesFlag        = "values"
	overwriteFlag     = "overwrite"
	inputFilePathFlag = "cli-input-yaml"

	includeStateMachineLogsFlag = "include-state-machine"
)

// Short flag names.
// A short flag only exists if the flag or flag set is mandatory by the command.
const (
	nameFlagShort     = "n"
	appFlagShort      = "a"
	envFlagShort      = "e"
	typeFlagShort     = "t"
	workloadFlagShort = "w"
	previousFlagShort = "p"

	dockerFileFlagShort        = "d"
	commandFlagShort           = "c"
	imageFlagShort             = "i"
	repoURLFlagShort           = "u"
	githubAccessTokenFlagShort = "t"
	gitBranchFlagShort         = "b"
	envsFlagShort              = "e"
	pipelineTypeShort          = "p"

	scheduleFlagShort = "s"
)

// Descriptions for flags.
var (
	svcTypeFlagDescription = fmt.Sprintf(`Type of service to create. Must be one of:
%s.`, strings.Join(quoteStringSlice(manifest.ServiceTypes()), ", "))
	imageFlagDescription = fmt.Sprintf(`The location of an existing Docker image.
Cannot be specified with --%s or --%s.`, dockerFileFlag, dockerFileContextFlag)
	dockerFileFlagDescription = fmt.Sprintf(`Path to the Dockerfile.
Cannot be specified with --%s.`, imageFlag)
	dockerFileContextFlagDescription = fmt.Sprintf(`Path to the Docker build context.
Cannot be specified with --%s.`, imageFlag)
	storageTypeFlagDescription = fmt.Sprintf(`Type of storage to add. Must be one of:
%s.`, strings.Join(quoteStringSlice(storageTypes), ", "))
	jobTypeFlagDescription = fmt.Sprintf(`Type of job to create. Must be one of:
%s.`, strings.Join(quoteStringSlice(manifest.JobTypes()), ", "))
	wkldTypeFlagDescription = fmt.Sprintf(`Type of job or svc to create. Must be one of:
%s.`, strings.Join(quoteStringSlice(manifest.WorkloadTypes()), ", "))

	clusterFlagDescription = fmt.Sprintf(`Optional. The short name or full ARN of the cluster to run the task in. 
Cannot be specified with --%s, --%s or --%s.`, appFlag, envFlag, taskDefaultFlag)
	acknowledgeSecretsAccessDescription = fmt.Sprintf(`Optional. Skip the confirmation question and grant access to the secrets specified by --secrets flag. 
This flag is useful only when '%s' flag is specified`, secretsFlag)
	subnetsFlagDescription = fmt.Sprintf(`Optional. The subnet IDs for the task to use. Can be specified multiple times.
Cannot be specified with --%s, --%s or --%s.`, appFlag, envFlag, taskDefaultFlag)
	securityGroupsFlagDescription = "Optional. Additional security group IDs for the task to use. Can be specified multiple times."
	taskRunDefaultFlagDescription = fmt.Sprintf(`Optional. Run tasks in default cluster and default subnets. 
Cannot be specified with --%s, --%s or --%s.`, appFlag, envFlag, subnetsFlag)
	taskExecDefaultFlagDescription = fmt.Sprintf(`Optional. Execute commands in running tasks in default cluster and default subnets. 
Cannot be specified with --%s or --%s.`, appFlag, envFlag)
	taskDeleteDefaultFlagDescription = fmt.Sprintf(`Optional. Delete a task which was launched in the default cluster and subnets.
Cannot be specified with --%s or --%s.`, appFlag, envFlag)
	taskEnvFlagDescription = fmt.Sprintf(`Optional. Name of the environment.
Cannot be specified with --%s, --%s or --%s.`, taskDefaultFlag, subnetsFlag, securityGroupsFlag)
	taskAppFlagDescription = fmt.Sprintf(`Optional. Name of the application.
Cannot be specified with --%s, --%s or --%s.`, taskDefaultFlag, subnetsFlag, securityGroupsFlag)
	osFlagDescription   = fmt.Sprintf(`Optional. Operating system of the task. Must be specified along with '%s'.`, archFlag)
	archFlagDescription = fmt.Sprintf(`Optional. Architecture of the task. Must be specified along with '%s'.`, osFlag)

	secretNameFlagDescription = fmt.Sprintf(`The name of the secret.
Mutually exclusive with the --%s flag.`, inputFilePathFlag)
	secretValuesFlagDescription = fmt.Sprintf(`Values of the secret in each environment. Specified as <environment>=<value> separated by commas.
Mutually exclusive with the --%s flag.`, inputFilePathFlag)
	secretInputFilePathFlagDescription = fmt.Sprintf(`Optional. A YAML file in which the secret values are specified.
Mutually exclusive with the -%s ,--%s and --%s flags.`, nameFlagShort, nameFlag, valuesFlag)

	repoURLFlagDescription = fmt.Sprintf(`The repository URL to trigger your pipeline.
Supported providers are: %s.`, strings.Join(manifest.PipelineProviders, ", "))
)

const (
	appFlagDescription            = "Name of the application."
	envFlagDescription            = "Name of the environment."
	svcFlagDescription            = "Name of the service."
	jobFlagDescription            = "Name of the job."
	workloadFlagDescription       = "Name of the service or job."
	nameFlagDescription           = "Name of the service, job, or task group."
	pipelineFlagDescription       = "Name of the pipeline."
	profileFlagDescription        = "Name of the profile."
	yesFlagDescription            = "Skips confirmation prompt."
	execYesFlagDescription        = "Optional. Whether to update the Session Manager Plugin."
	jsonFlagDescription           = "Optional. Output in JSON format."
	forceFlagDescription          = "Optional. Force a new service deployment using the existing image."
	forceEnvDeployFlagDescription = "Optional. Force update the environment stack template."
	noRollbackFlagDescription     = `Optional. Disable automatic stack 
rollback in case of deployment failure.
We do not recommend using this flag for a
production environment.`
	manifestFlagDescription    = "Optional. Output the manifest file used for the deployment."
	svcManifestFlagDescription = `Optional. Name of the environment in which the service was deployed;
output the manifest file used for that deployment.`

	imageTagFlagDescription     = `Optional. The container image tag.`
	resourceTagsFlagDescription = `Optional. Labels with a key and value separated by commas.
Allows you to categorize resources.`
	stackOutputDirFlagDescription = "Optional. Writes the stack template and template configuration to a directory."
	uploadAssetsFlagDescription   = `Optional. Whether to upload assets (container images, Lambda functions, etc.).
Uploaded asset locations are filled in the template configuration.`
	prodEnvFlagDescription = "If the environment contains production services."

	limitFlagDescription = `Optional. The maximum number of log events returned. Default is 10
unless any time filtering flags are set.`
	lastFlagDescription = `Optional. The number of executions of the scheduled job for which
logs should be shown.`
	followFlagDescription   = "Optional. Specifies if the logs should be streamed."
	previousFlagDescription = "Optional. Print logs for the last stopped task if exists."
	sinceFlagDescription    = `Optional. Only return logs newer than a relative duration like 5s, 2m, or 3h.
Defaults to all logs. Only one of start-time / since may be used.`
	startTimeFlagDescription = `Optional. Only return logs after a specific date (RFC3339).
Defaults to all logs. Only one of start-time / since may be used.`
	endTimeFlagDescription = `Optional. Only return logs before a specific date (RFC3339).
Defaults to all logs. Only one of end-time / follow may be used.`
	tasksLogsFlagDescription               = "Optional. Only return logs from specific task IDs."
	includeStateMachineLogsFlagDescription = "Optional. Include logs from the state machine executions."
	logGroupFlagDescription                = "Optional. Only return logs from specific log group."
	containerLogFlagDescription            = "Optional. Return only logs from a specific container."

	deployTestFlagDescription        = `Deploy your service or job to a "test" environment.`
	githubURLFlagDescription         = "(Deprecated.) Use '--url' instead. Repository URL to trigger your pipeline."
	githubAccessTokenFlagDescription = "GitHub personal access token for your repository."
	gitBranchFlagDescription         = "Branch used to trigger your pipeline."
	pipelineEnvsFlagDescription      = "Environments to add to the pipeline."
	pipelineTypeFlagDescription      = `The type of pipeline. Must be either "Workloads" or "Environments".`
	domainNameFlagDescription        = "Optional. Your existing custom domain name."
	envResourcesFlagDescription      = "Optional. Show the resources in your environment."
	svcResourcesFlagDescription      = "Optional. Show the resources in your service."
	pipelineResourcesFlagDescription = "Optional. Show the resources in your pipeline."
	localSvcFlagDescription          = "Only show services in the workspace."
	localJobFlagDescription          = "Only show jobs in the workspace."
	localPipelineFlagDescription     = "Only show pipelines in the workspace."
	deleteSecretFlagDescription      = "Deletes AWS Secrets Manager secret associated with a pipeline source repository."
	svcPortFlagDescription           = "The port on which your service listens."

	noSubscriptionFlagDescription  = "Optional. Turn off selection for adding subscriptions for worker services."
	subscribeTopicsFlagDescription = `Optional. SNS Topics to subscribe to from other services in your application.
Must be of format '<svcName>:<topicName>'`

	storageFlagDescription             = "Name of the storage resource to create."
	storageWorkloadFlagDescription     = "Name of the service or job to associate with storage."
	storagePartitionKeyFlagDescription = `Partition key for the DDB table.
Must be of the format '<keyName>:<dataType>'.`
	storageSortKeyFlagDescription = `Optional. Sort key for the DDB table.
Must be of the format '<keyName>:<dataType>'.`
	storageNoSortFlagDescription    = "Optional. Skip configuring sort keys."
	storageNoLSIFlagDescription     = `Optional. Don't ask about configuring alternate sort keys.`
	storageLSIConfigFlagDescription = `Optional. Attribute to use as an alternate sort key. May be specified up to 5 times.
Must be of the format '<keyName>:<dataType>'.`
	storageRDSEngineFlagDescription = `The database engine used in the cluster.
Must be either "MySQL" or "PostgreSQL".`
	storageRDSInitialDBFlagDescription      = "The initial database to create in the cluster."
	storageRDSParameterGroupFlagDescription = "Optional. The name of the parameter group to associate with the cluster."

	countFlagDescription         = "Optional. The number of tasks to set up."
	cpuFlagDescription           = "Optional. The number of CPU units to reserve for each task."
	memoryFlagDescription        = "Optional. The amount of memory to reserve in MiB for each task."
	taskRoleFlagDescription      = "Optional. The ARN of the role for the task to use."
	executionRoleFlagDescription = "Optional. The ARN of the role that grants the container agent permission to make AWS API calls."
	envVarsFlagDescription       = "Optional. Environment variables specified by key=value separated by commas."
	envFileFlagDescription       = `Optional. A path to an environment variable (.env) file. 
Each line should be of the form of VARIABLE=VALUE. 
Values specified with --env-vars take precedence over --env-file.`
	secretsFlagDescription    = "Optional. Secrets to inject into the container. Specified by key=value separated by commas."
	runCommandFlagDescription = `Optional. The command that is passed to "docker run" to override the default command.`
	entrypointFlagDescription = `Optional. The entrypoint that is passed to "docker run" to override the default entrypoint.`
	taskGroupFlagDescription  = `Optional. The group name of the task. 
Tasks with the same group name share the same set of resources. 
(default directory name)`
	taskImageTagFlagDescription    = `Optional. The container image tag in addition to "latest".`
	generateCommandFlagDescription = `Optional. Generate a command with a pre-filled value for each flag.
To use it for an ECS service, specify --generate-cmd <cluster name>/<service name>.
Alternatively, if the service or job is created with Copilot, specify --generate-cmd <application>/<environment>/<service or job name>.
Cannot be specified with any other flags.`

	vpcIDFlagDescription              = "Optional. Use an existing VPC ID."
	publicSubnetsFlagDescription      = "Optional. Use existing public subnet IDs."
	privateSubnetsFlagDescription     = "Optional. Use existing private subnet IDs."
	certsFlagDescription              = "Optional. Apply existing ACM certificates to the internet-facing load balancer."
	internalALBSubnetsFlagDescription = `Optional. Specify subnet IDs for an internal load balancer.
By default, the load balancer will be placed in your private subnets.
Cannot be specified with --default-config or any of the --override flags.`
	allowVPCIngressFlagDescription = `Optional. Allow internal ALB ingress from port 80 and/or port 443.`
	overrideVPCCIDRFlagDescription = `Optional. Global CIDR to use for VPC.
(default 10.0.0.0/16)`
	overrideAZsFlagDescription = `Optional. Availability Zone names.
(default 2 random AZs)`
	overridePublicSubnetCIDRsFlagDescription = `Optional. CIDR to use for public subnets. 
(default 10.0.0.0/24,10.0.1.0/24)`
	overridePrivateSubnetCIDRsFlagDescription = `Optional. CIDR to use for private subnets.
(default 10.0.2.0/24,10.0.3.0/24)`

	enableContainerInsightsFlagDescription = "Optional. Enable CloudWatch Container Insights."

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
For example: "0 * * * *", "@daily", "@weekly", "@every 1h30m".
AWS Schedule Expressions of the form "rate(10 minutes)" or "cron(0 12 L * ? 2021)"
are also accepted.`

	upgradeAllEnvsDescription = "Optional. Upgrade all environments."

	taskIDFlagDescription      = "Optional. ID of the task you want to exec in."
	execCommandFlagDescription = `Optional. The command that is passed to a running container.`
	containerFlagDescription   = "Optional. The specific container you want to exec in. By default the first essential container will be used."

	secretOverwriteFlagDescription = "Optional. Whether to overwrite an existing secret."
)
