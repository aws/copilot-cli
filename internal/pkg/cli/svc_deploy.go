// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/spf13/cobra"

	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
)

type deployWkldVars struct {
	packageSvcVars

	resourceTags    map[string]string
	forceNewUpdate  bool // NOTE: this variable is not applicable for a job workload currently.
	disableRollback bool

	// To facilitate unit tests.
	clientConfigured bool
}

type deploySvcOpts struct {
	deployWkldVars

	// deps
	packageSvcOpts packageSvcOpts
	cmd            execRunner
	spinner        progress

	// cached variables
	deployRecs []string
}

func newSvcDeployOpts(vars deployWkldVars) (*deploySvcOpts, error) {
	packageSvcOpts, err := newPackageSvcOpts(vars.packageSvcVars, "svc deploy")
	if err != nil {
		return nil, err
	}

	return &deploySvcOpts{
		deployWkldVars: vars,
		packageSvcOpts: *packageSvcOpts,
		spinner:        termprogress.NewSpinner(log.DiagnosticWriter),
		cmd:            exec.NewCmd(),
	}, nil
}

// Validate returns an error for any invalid optional flags.
func (o *deploySvcOpts) Validate() error {
	return o.packageSvcOpts.Validate()
}

// Ask prompts for and validates any required flags.
func (o *deploySvcOpts) Ask() error {
	return o.packageSvcOpts.Ask()
}

// Execute builds and pushes the container image for the service,
func (o *deploySvcOpts) Execute() error {
	deployer, err := o.packageSvcOpts.Deployer()
	if err != nil {
		return err
	}

	if err := deployer.UploadArtifacts(); err != nil {
		return fmt.Errorf("upload artifacts: %w", err)
	}

	// generate stacks
	stack, err := deployer.Stack(clideploy.StackRuntimeConfiguration{
		RootUserARN: o.packageSvcOpts.rootUserARN,
		Tags:        tags.Merge(o.packageSvcOpts.targetApp.Tags, o.resourceTags),
	})
	if err != nil {
		return fmt.Errorf("generate stack: %w", err)
	}

	err = deployer.Deploy(stack, clideploy.DeployOpts{
		ForceNewUpdate:  o.forceNewUpdate,
		DisableRollback: o.disableRollback,
	})
	if err != nil {
		if o.disableRollback {
			// stackName := stack.NameForService(o.packageSvcOpts.targetApp.Name, o.packageSvcOpts.targetEnv.Name, o.name)
			stackName := "TODO"
			rollbackCmd := fmt.Sprintf("aws cloudformation rollback-stack --stack-name %s --role-arn %s", stackName, o.packageSvcOpts.targetEnv.ExecutionRoleARN)
			log.Infof(`It seems like you have disabled automatic stack rollback for this deployment. To debug, you can:
	* Run %s to inspect the service log.
	* Visit the AWS console to inspect the errors.
	After fixing the deployment, you can:
	1. Run %s to rollback the deployment.
	2. Run %s to make a new deployment.
`, color.HighlightCode("copilot svc logs"), color.HighlightCode(rollbackCmd), color.HighlightCode("copilot svc deploy"))
		}
		return fmt.Errorf("deploy service %s to environment %s: %w", o.name, o.envName, err)
	}

	o.deployRecs = deployer.RecommendActions()
	log.Successf("Deployed service %s.\n", color.HighlightUserInput(o.name))
	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *deploySvcOpts) RecommendActions() error {
	var recommendations []string
	uriRecs, err := o.uriRecommendedActions()
	if err != nil {
		return err
	}
	recommendations = append(recommendations, uriRecs...)
	recommendations = append(recommendations, o.deployRecs...)
	recommendations = append(recommendations, o.publishRecommendedActions()...)
	logRecommendedActions(recommendations)
	return nil
}

type workloadManifestInput struct {
	name         string
	appName      string
	envName      string
	ws           wsWlDirReader
	interpolator interpolator
	sess         *session.Session
	unmarshal    func([]byte) (manifest.DynamicWorkload, error)
}

func workloadManifest(in *workloadManifestInput) (manifest.DynamicWorkload, error) {
	raw, err := in.ws.ReadWorkloadManifest(in.name)
	if err != nil {
		return nil, fmt.Errorf("read manifest file for %s: %w", in.name, err)
	}
	interpolated, err := in.interpolator.Interpolate(string(raw))
	if err != nil {
		return nil, fmt.Errorf("interpolate environment variables for %s manifest: %w", in.name, err)
	}
	mft, err := in.unmarshal([]byte(interpolated))
	if err != nil {
		return nil, fmt.Errorf("unmarshal service %s manifest: %w", in.name, err)
	}
	envMft, err := mft.ApplyEnv(in.envName)
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %w", in.envName, err)
	}
	if err := envMft.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest against environment %q: %w", in.envName, err)
	}
	if err := envMft.Load(in.sess); err != nil {
		return nil, fmt.Errorf("load dynamic content: %w", err)
	}
	return envMft, nil
}

func validateWorkloadManifestCompatibilityWithEnv(ws wsEnvironmentsLister, env versionCompatibilityChecker, mft manifest.DynamicWorkload, envName string) error {
	currVersion, err := env.Version()
	if err != nil {
		return fmt.Errorf("get environment %q version: %w", envName, err)
	}
	if currVersion == deploy.EnvTemplateVersionBootstrap {
		return fmt.Errorf(`cannot deploy a service to an undeployed environment. Please run "copilot env deploy --name %s" to deploy the environment first`, envName)
	}
	availableFeatures, err := env.AvailableFeatures()
	if err != nil {
		return fmt.Errorf("get available features of the %s environment stack: %w", envName, err)
	}
	exists := struct{}{}
	available := make(map[string]struct{})
	for _, f := range availableFeatures {
		available[f] = exists
	}

	features := mft.RequiredEnvironmentFeatures()
	for _, f := range features {
		if _, ok := available[f]; !ok {
			logMsg := fmt.Sprintf(`Your manifest configuration requires your environment %q to have the feature %q available.`, envName, template.FriendlyEnvFeatureName(f))
			if v := template.LeastVersionForFeature(f); v != "" {
				logMsg += fmt.Sprintf(` The least environment version that supports the feature is %s.`, v)
			}
			logMsg += fmt.Sprintf(" Your environment is on %s.", currVersion)
			log.Errorln(logMsg)
			return &errFeatureIncompatibleWithEnvironment{
				ws:             ws,
				missingFeature: f,
				envName:        envName,
				curVersion:     currVersion,
			}
		}
	}
	return nil
}

func (o *deploySvcOpts) uriRecommendedActions() ([]string, error) {
	type reachable interface {
		Port() (uint16, bool)
	}
	mft, ok := o.packageSvcOpts.appliedDynamicMft.Manifest().(reachable)
	if !ok {
		return nil, nil
	}
	if _, ok := mft.Port(); !ok { // No exposed port.
		return nil, nil
	}

	describer, err := describe.NewReachableService(o.appName, o.name, o.packageSvcOpts.store)
	if err != nil {
		return nil, err
	}
	uri, err := describer.URI(o.envName)
	if err != nil {
		return nil, fmt.Errorf("get uri for environment %s: %w", o.envName, err)
	}

	network := "over the internet."
	switch uri.AccessType {
	case describe.URIAccessTypeInternal:
		network = "from your internal network."
	case describe.URIAccessTypeServiceDiscovery:
		network = "with service discovery."
	case describe.URIAccessTypeServiceConnect:
		network = "with service connect."
	}

	return []string{
		fmt.Sprintf("You can access your service at %s %s", color.HighlightResource(uri.URI), network),
	}, nil
}

func (o *deploySvcOpts) publishRecommendedActions() []string {
	type publisher interface {
		Publish() []manifest.Topic
	}
	mft, ok := o.packageSvcOpts.appliedDynamicMft.Manifest().(publisher)
	if !ok {
		return nil
	}
	if topics := mft.Publish(); len(topics) == 0 {
		return nil
	}

	return []string{
		fmt.Sprintf(`Update %s's code to leverage the injected environment variable "COPILOT_SNS_TOPIC_ARNS".
    In JavaScript you can write %s.`,
			o.name,
			color.HighlightCode("const {<topicName>} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS)")),
	}
}

type errFeatureIncompatibleWithEnvironment struct {
	ws             wsEnvironmentsLister
	missingFeature string
	envName        string
	curVersion     string
}

func (e *errFeatureIncompatibleWithEnvironment) Error() string {
	if e.curVersion == "" {
		return fmt.Sprintf("environment %q is not on a version that supports the %q feature", e.envName, template.FriendlyEnvFeatureName(e.missingFeature))
	}
	return fmt.Sprintf("environment %q is on version %q which does not support the %q feature", e.envName, e.curVersion, template.FriendlyEnvFeatureName(e.missingFeature))
}

// RecommendActions returns recommended actions to be taken after the error.
// Implements main.actionRecommender interface.
func (e *errFeatureIncompatibleWithEnvironment) RecommendActions() string {
	envs, _ := e.ws.ListEnvironments() // Best effort try to detect if env manifest exists.
	for _, env := range envs {
		if e.envName == env {
			return fmt.Sprintf("You can upgrade the %q environment template by running %s.", e.envName, color.HighlightCode(fmt.Sprintf("copilot env deploy --name %s", e.envName)))
		}
	}
	msgs := []string{
		"You can upgrade your environment template by running:",
		fmt.Sprintf("1. Create the directory to store your environment manifest %s.",
			color.HighlightCode(fmt.Sprintf("mkdir -p %s", filepath.Join("copilot", "environments", e.envName)))),
		fmt.Sprintf("2. Generate the manifest %s.",
			color.HighlightCode(fmt.Sprintf("copilot env show -n %s --manifest > %s", e.envName, filepath.Join("copilot", "environments", e.envName, "manifest.yml")))),
		fmt.Sprintf("3. Deploy the environment stack %s.",
			color.HighlightCode(fmt.Sprintf("copilot env deploy --name %s", e.envName))),
	}
	return strings.Join(msgs, "\n")

}

// buildSvcDeployCmd builds the `svc deploy` subcommand.
func buildSvcDeployCmd() *cobra.Command {
	vars := deployWkldVars{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys a service to an environment.",
		Long:  `Deploys a service to an environment.`,
		Example: `
  Deploys a service named "frontend" to a "test" environment.
  /code $ copilot svc deploy --name frontend --env test
  Deploys a service with additional resource tags.
  /code $ copilot svc deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcDeployOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.tag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	cmd.Flags().BoolVar(&vars.forceNewUpdate, forceFlag, false, forceFlagDescription)
	cmd.Flags().BoolVar(&vars.disableRollback, noRollbackFlag, false, noRollbackFlagDescription)
	cmd.Flags().BoolVar(&vars.showDiff, diffFlag, false, noRollbackFlagDescription)

	return cmd
}
