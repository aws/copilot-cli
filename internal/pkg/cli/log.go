package cli

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// logOpts contains the fields to collect to delete a secret.
type logOpts struct {
	appName string
	envName string
	start   string
	tail    bool

	logManager     archer.LogManager
	projectService projectService
	storeReader    storeReader

	ws archer.Workspace

	*GlobalOpts
}

// Validate returns an error if the values provided by the user are invalid.
func (o *logOpts) Validate() error {
	if o.ProjectName() != "" {
		_, err := o.storeReader.GetProject(o.ProjectName())
		if err != nil {
			return err
		}
	}
	if o.appName != "" {
		_, err := o.storeReader.GetApplication(o.ProjectName(), o.appName)
		if err != nil {
			return err
		}
	}
	if o.envName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}

	return nil
}

func (o *logOpts) validateEnvName() error {
	if _, err := o.targetEnv(); err != nil {
		return err
	}
	return nil
}

func (o *logOpts) targetEnv() (*archer.Environment, error) {
	env, err := o.projectService.GetEnvironment(o.ProjectName(), o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s from metadata store: %w", o.envName, err)
	}
	return env, nil
}

// Ask asks for fields that are required but not passed in.
func (o *logOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	if err := o.askAppName(); err != nil {
		return err
	}
	return o.askEnvName()
}

// Execute displays the logs.
func (o *logOpts) Execute() error {
	var startTime int64

	logID := fmt.Sprintf("%s-%s-%s", o.projectName, o.envName, o.appName)

	if o.start == "" {
		startTime = time.Now().Add(-24 * time.Hour).UnixNano() / 1e6 // msec
	} else {
		re := regexp.MustCompile(`^([0-9]*)([A-Za-z])$`)
		results := re.FindStringSubmatch(o.start)
		if len(results) != 3 {
			return errors.New("'start' must be in the format: 24h")
		}
		startNum, _ := strconv.Atoi(results[1])
		startType := results[2]

		var then time.Time
		now := time.Now()

		switch strings.ToLower(startType) {
		case "m":
			then = now.Add(time.Duration(-startNum) * time.Minute)
		case "h":
			then = now.Add(time.Duration(-startNum) * time.Hour)
		case "d":
			then = now.AddDate(0, 0, -startNum)
		case "w":
			then = now.AddDate(0, 0, -startNum * 7)
		default:
			return errors.New("'start' type must be: m, h, d, w")
		}

		startTime = then.UnixNano() / 1e6 // msec
	}

	startTime, err := o.showEntries(logID, startTime)
	if err != nil {
		return err
	}
	if o.tail {
		// listen for Enter key to exit
		go func() {
			fmt.Scanln()
			os.Exit(0)
		}()

		for {
			time.Sleep(10 * time.Second)
			startTime, err = o.showEntries(logID, startTime)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// returns the endTime to serve as the startTime for the next iteration
func (o *logOpts) showEntries(logID string, startTime int64) (int64, error) {
	entries, endTime, err := o.logManager.GetLog(logID, startTime)
	if err != nil {
		return 0, err
	}
	o.print(entries)

	return endTime, nil
}

func (o *logOpts) print(entries *[]archer.LogEntry) {
	for _, e := range *entries {
		sec := e.Timestamp / 1000
		localTime := time.Unix(sec, 0).Local().Format(time.RFC3339)

		fmt.Printf("%s %s %s\n", color.HighlightLogTimestamp(localTime),
			color.HighlightLogStreamName(e.StreamName), e.Message)
	}
}

func (o *logOpts) askProject() error {
	if o.ProjectName() != "" {
		return nil
	}
	projNames, err := o.retrieveProjects()
	if err != nil {
		return err
	}
	if len(projNames) == 0 {
		log.Infoln("There are no projects to select.")
	}
	proj, err := o.prompt.SelectOne(
		"Which project:",
		applicationShowProjectNameHelpPrompt,
		projNames,
	)
	if err != nil {
		return fmt.Errorf("selecting projects: %w", err)
	}
	o.projectName = proj

	return nil
}

func (o *logOpts) askEnvName() error {
	if o.envName != "" {
		return nil
	}

	envs, err := o.projectService.ListEnvironments(o.ProjectName())
	if err != nil {
		return fmt.Errorf("get environments for project %s from metadata store: %w", o.ProjectName(), err)
	}
	if len(envs) == 0 {
		log.Infof("Couldn't find any environments associated with project %s, try initializing one: %s\n",
			color.HighlightUserInput(o.ProjectName()),
			color.HighlightCode("dw_run.sh env init"))
		return fmt.Errorf("no environments found in project %s", o.ProjectName())
	}
	if len(envs) == 1 {
		o.envName = envs[0].Name
		log.Infof("Only found one environment, defaulting to: %s\n", color.HighlightUserInput(o.envName))
		return nil
	}

	var names []string
	for _, env := range envs {
		names = append(names, env.Name)
	}

	selectedEnvName, err := o.prompt.SelectOne("Select an environment", "", names)
	if err != nil {
		return fmt.Errorf("select env name: %w", err)
	}
	o.envName = selectedEnvName
	return nil
}

func (o *logOpts) askAppName() error {
	if o.appName != "" {
		return nil
	}
	appNames, err := o.retrieveApplications()
	if err != nil {
		return err
	}
	if len(appNames) == 0 {
		log.Infof("No applications found in project '%s'\n.", o.ProjectName())
		return nil
	}
	appName, err := o.prompt.SelectOne(
		fmt.Sprintf("Which app:"),
		"The app this secret belongs to.",
		appNames,
	)
	if err != nil {
		return fmt.Errorf("selecting applications for project %s: %w", o.ProjectName(), err)
	}
	o.appName = appName

	return nil
}

func (o *logOpts) retrieveProjects() ([]string, error) {
	projs, err := o.storeReader.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	projNames := make([]string, len(projs))
	for ind, proj := range projs {
		projNames[ind] = proj.Name
	}
	return projNames, nil
}

func (o *logOpts) retrieveApplications() ([]string, error) {
	apps, err := o.storeReader.ListApplications(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("listing applications for project %s: %w", o.ProjectName(), err)
	}
	appNames := make([]string, len(apps))
	for ind, app := range apps {
		appNames[ind] = app.Name
	}
	return appNames, nil
}

// BuildLogCmd displays the log entries for an app.
func BuildLogCmd() *cobra.Command {
	opts := logOpts{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:     "log",
		Short:   "Log commands.",
		Example: `
  Displays the log entries for the last 24 hours (default value).
  /code $ dw_run.sh log

  Displays the log entries for the last 14 days.
  /code $ dw_run.sh log --start 14d

  The 'start' parameter accepts a number and a range type.
  Range types: m -> minutes, h -> hours, d -> days, w -> weeks

  Displays the log entries for the last 24 hours and will show any new entries as they come in.
  /code $ dw_run.sh log --tail

  Displays the log entries for the last 20 minutes and will show any new entries as they come in.
  /code $ dw_run.sh log --start 20m --tail
`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			store, err := store.New()
			if err != nil {
				return fmt.Errorf("connect to environment datastore: %w", err)
			}
			ws, err := workspace.New()
			if err != nil {
				return fmt.Errorf("new workspace: %w", err)
			}
			opts.ws = ws
			opts.logManager = store
			opts.projectService = store
			opts.storeReader = store

			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}

	cmd.Flags().StringVarP(&opts.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&opts.start, "start", "s", "", "How far back to look, e.g. `20m`.")
	cmd.Flags().BoolVar(&opts.tail, "tail", false,"Continuously show new entries.")
	cmd.Flags().StringVarP(&opts.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringP(projectFlag, projectFlagShort, "dw-run" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.Flags().Lookup(projectFlag))

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}

	return cmd
}
