// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

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
	localAppFlag          = "local"
	deleteSecretFlag      = "delete-secret"
	appPortFlag           = "port"
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
	githubURLFlagShort         = "u"
	githubAccessTokenFlagShort = "t"
	gitBranchFlagShort         = "b"
	envsFlagShort              = "e"
)

// Descriptions for flags.
var (
	appTypeFlagDescription = fmt.Sprintf(`Type of application to create. Must be one of:
%s`, strings.Join(quoteAll(manifest.AppTypes), ", "))
)

const (
	projectFlagDescription = "Name of the project."
	appFlagDescription     = "Name of the application."
	envFlagDescription     = "Name of the environment."
	profileFlagDescription = "Name of the profile."
	yesFlagDescription     = "Skips confirmation prompt."
	jsonFlagDescription    = "Optional. Outputs in JSON format."

	dockerFileFlagDescription   = "Path to the Dockerfile."
	imageTagFlagDescription     = `Optional. The application's image tag.`
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
	deployTestFlagDescription        = `Deploy your application to a "test" environment.`
	githubURLFlagDescription         = "GitHub repository URL for your application."
	githubAccessTokenFlagDescription = "GitHub personal access token for your repository."
	gitBranchFlagDescription         = "Branch used to trigger your pipeline."
	pipelineEnvsFlagDescription      = "Environments to add to the pipeline."
	domainNameFlagDescription        = "Optional. Your existing custom domain name."
	resourcesFlagDescription         = "Optional. Show the resources of your application."
	localAppFlagDescription          = "Only show applications in the current directory."
	envProfilesFlagDescription       = "Optional. Environments and the profile to use to delete the environment."
	deleteSecretFlagDescription      = "Deletes AWS Secrets Manager secret associated with a pipeline source repository."
	appPortFlagDescription           = "Optional. The port on which your Dockerfile listens."
)

func quoteAll(elems []string) []string {
	quotedElems := make([]string, len(elems))
	for i, el := range elems {
		quotedElems[i] = strconv.Quote(el)
	}
	return quotedElems
}
