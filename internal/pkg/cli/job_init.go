// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding"
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	fmtJobInitNameHelpPrompt = `The name will uniquely identify this job within your app %s.
Deployed resources (such as your job, logs) will contain this job's name and be tagged with it.`

	jobInitDockerfileHelpPrompt = "Dockerfile to use for building your job's container image."
)

// const (
// 	fmtAddJobToAppStart    = "Creating ECR repositories for job %s."
// 	fmtAddJobToAppFailed   = "Failed to create ECR repositories for job %s.\n"
// 	fmtAddJobToAppComplete = "Created ECR repositories for job %s.\n"
// )

type initJobVars struct {
	*GlobalOpts
	Name           string
	DockerfilePath string
}

type initJobOpts struct {
	initJobVars

	// Interfaces to interact with dependencies.
	fs          afero.Fs
	ws          svcManifestWriter
	store       store
	appDeployer appDeployer
	prog        progress

	// Outputs stored on successful actions.
	manifestPath string
}

func newInitJobOpts(vars initJobVars) (*initJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}

	p := sessions.NewProvider()
	sess, err := p.Default()
	if err != nil {
		return nil, err
	}

	return &initJobOpts{
		initJobVars: vars,

		fs:          &afero.Afero{Fs: afero.NewOsFs()},
		store:       store,
		ws:          ws,
		appDeployer: cloudformation.New(sess),
		prog:        termprogress.NewSpinner(),
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *initJobOpts) Validate() error {
	if o.AppName() == "" {
		return errNoAppInWorkspace
	}
	if o.Name != "" {
		if err := validateSvcName(o.Name); err != nil {
			return err
		}
	}
	if o.DockerfilePath != "" {
		if _, err := o.fs.Stat(o.DockerfilePath); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *initJobOpts) Ask() error {
	if err := o.askJobName(); err != nil {
		return err
	}
	if err := o.askDockerfile(); err != nil {
		return err
	}

	return nil
}

// Execute writes the service's manifest file and stores the service in SSM.
func (o *initJobOpts) Execute() error {
	app, err := o.store.GetApplication(o.AppName())
	if err != nil {
		return fmt.Errorf("get application %s: %w", o.AppName(), err)
	}

	manifestPath, err := o.createManifest()
	if err != nil {
		return err
	}
	o.manifestPath = manifestPath

	o.prog.Start(fmt.Sprintf(fmtAddSvcToAppStart, o.Name))
	if err := o.appDeployer.AddServiceToApp(app, o.Name); err != nil {
		o.prog.Stop(log.Serrorf(fmtAddSvcToAppFailed, o.Name))
		return fmt.Errorf("add job %s to application %s: %w", o.Name, o.AppName(), err)
	}
	o.prog.Stop(log.Ssuccessf(fmtAddSvcToAppComplete, o.Name))

	if err := o.store.CreateService(&config.Service{
		App:  o.AppName(),
		Name: o.Name,
		Type: "Scheduled Job",
	}); err != nil {
		return fmt.Errorf("saving service %s: %w", o.Name, err)
	}
	return nil
}

func (o *initJobOpts) createManifest() (string, error) {
	manifest, err := o.newManifest()
	if err != nil {
		return "", err
	}
	var manifestExists bool
	manifestPath, err := o.ws.WriteServiceManifest(manifest, o.Name)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return "", err
		}
		manifestExists = true
		manifestPath = e.FileName
	}
	manifestPath, err = relPath(manifestPath)
	if err != nil {
		return "", err
	}

	manifestMsgFmt := "Wrote the manifest for job %s at %s\n"
	if manifestExists {
		manifestMsgFmt = "Manifest file for job %s already exists at %s, skipping writing it.\n"
	}
	log.Successf(manifestMsgFmt, color.HighlightUserInput(o.Name), color.HighlightResource(manifestPath))
	log.Infoln(color.Help("Your manifest contains configurations like your container size and retry logic"))
	log.Infoln()

	return manifestPath, nil
}

func (o *initJobOpts) newManifest() (encoding.BinaryMarshaler, error) {
	return &manifest.BackendService{}, nil
}

func (o *initJobOpts) askJobName() error {
	if o.Name != "" {
		return nil
	}

	name, err := o.prompt.Get(
		fmt.Sprintf(fmtSvcInitSvcNamePrompt, color.Emphasize("name"), color.HighlightUserInput("scheduled job")),
		fmt.Sprintf(fmtJobInitNameHelpPrompt, o.AppName()),
		validateSvcName,
		prompt.WithFinalMessage("Job name:"))
	if err != nil {
		return fmt.Errorf("get job name: %w", err)
	}
	o.Name = name
	return nil
}

// askDockerfile prompts for the Dockerfile by looking at sub-directories with a Dockerfile.
// If the user chooses to enter a custom path, then we prompt them for the path.
func (o *initJobOpts) askDockerfile() error {
	if o.DockerfilePath != "" {
		return nil
	}

	// TODO https://github.com/aws/copilot-cli/issues/206
	dockerfiles, err := listDockerfiles(o.fs, ".")
	if err != nil {
		return err
	}

	sel, err := o.prompt.SelectOne(
		fmt.Sprintf(fmtSvcInitDockerfilePrompt, color.Emphasize("Dockerfile"), color.HighlightUserInput(o.Name)),
		jobInitDockerfileHelpPrompt,
		dockerfiles,
		prompt.WithFinalMessage("Dockerfile:"),
	)
	if err != nil {
		return fmt.Errorf("select Dockerfile: %w", err)
	}

	o.DockerfilePath = sel

	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *initJobOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Update your manifest %s to change the defaults.", color.HighlightResource(o.manifestPath)),
		fmt.Sprintf("Run %s to deploy your job to a %s environment.",
			color.HighlightCode(fmt.Sprintf("copilot job deploy --name %s --env %s", o.Name, defaultEnvironmentName)),
			defaultEnvironmentName),
	}
}

// BuildJobInitCmd builds the command for creating a new service.
func BuildJobInitCmd() *cobra.Command {
	vars := initJobVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new scheduled job in an application.",
		Example: `
  Create a "reaper" scheduled task to run once per day.
  /code $ copilot job init --name reaper --dockerfile ./frontend/Dockerfile --rate "1 day"

  Create a "report-generator" scheduled task with retries.
  /code $ copilot job init --name report-generator --rate "1 day" --retries 3 --timeout `,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitJobOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil { // validate flags
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}
			log.Infoln("Recommended follow-up actions:")
			for _, followup := range opts.RecommendedActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.Name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.DockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)

	requiredFlags := pflag.NewFlagSet("Required Flags", pflag.ContinueOnError)
	requiredFlags.AddFlag(cmd.Flags().Lookup(nameFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(dockerFileFlag))

	cmd.Annotations = map[string]string{
		// The order of the sections we want to display.
		"sections": fmt.Sprintf(`Required,%s`, strings.Join(manifest.ServiceTypes, ",")),
		"Required": requiredFlags.FlagUsages(),
	}
	cmd.SetUsageTemplate(`{{h1 "Usage"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{$annotations := .Annotations}}{{$sections := split .Annotations.sections ","}}{{if gt (len $sections) 0}}

{{range $i, $sectionName := $sections}}{{h1 (print $sectionName " Flags")}}
{{(index $annotations $sectionName) | trimTrailingWhitespaces}}{{if ne (inc $i) (len $sections)}}

{{end}}{{end}}{{end}}{{if .HasAvailableInheritedFlags}}

{{h1 "Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{h1 "Examples"}}{{code .Example}}{{end}}
`)
	return cmd
}
