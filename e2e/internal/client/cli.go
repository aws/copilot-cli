package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

// CLI is a wrapper around os.execs
type CLI struct {
	path string
}

// ProjectInitRequest contains the parameters for calling ecs project init
type ProjectInitRequest struct {
	ProjectName string
	Domain      string
}

// InitRequest contains the parameters for calling ecs init
type InitRequest struct {
	ProjectName string
	AppName     string
	Deploy      bool
	ImageTag    string
	Dockerfile  string
	AppType     string
}

// EnvInitRequest contains the parameters for calling ecs env init
type EnvInitRequest struct {
	ProjectName string
	EnvName     string
	Profile     string
	Prod        bool
}

// AppInitRequest contains the parameters for calling ecs app init
type AppInitRequest struct {
	AppName    string
	AppType    string
	Dockerfile string
}

// AppShowRequest contains the parameters for calling ecs app show
type AppShowRequest struct {
	ProjectName string
	AppName     string
}

// AppLogsRequest contains the parameters for calling ecs app logs
type AppLogsRequest struct {
	ProjectName string
	AppName     string
	EnvName     string
	Since       string
}

// AppDeployInput contains the parameters for calling ecs app deploy
type AppDeployInput struct {
	AppName  string
	EnvName  string
	ImageTag string
}

// NewCLI returns a wrapper around CLI
func NewCLI() (*CLI, error) {
	// These tests should be run in a dockerfile so that
	// your file system and docker image repo isn't polluted
	// with test data and files. Since this is going to run
	// from Docker, the binary will localted in the root bin.
	cliPath := filepath.Join("/", "bin", "ecs-preview")
	if _, err := os.Stat(cliPath); err != nil {
		return nil, err
	}

	return &CLI{
		path: cliPath,
	}, nil
}

/*Help runs
ecs-preview --help
*/
func (cli *CLI) Help() (string, error) {
	return cli.exec(exec.Command(cli.path, "--help"))
}

/*Version runs:
ecs-preview --version
*/
func (cli *CLI) Version() (string, error) {
	return cli.exec(exec.Command(cli.path, "--version"))
}

/*Init runs:
ecs-preview init
	--project $p
	--app $a
	--app-type $type
	--tag $t
	--dockerfile $d
	--deploy (optionally)
*/
func (cli *CLI) Init(opts *InitRequest) (string, error) {
	var deployOption string

	if opts.Deploy {
		deployOption = "--deploy"
	}

	return cli.exec(
		exec.Command(cli.path, "init",
			"--project", opts.ProjectName,
			"--app", opts.AppName,
			"--app-type", opts.AppType,
			"--tag", opts.ImageTag,
			"--dockerfile", opts.Dockerfile,
			deployOption))
}

/*AppInit runs:
ecs-preview app init
	--name $n
	--app-type $t
	--dockerfile $d
*/
func (cli *CLI) AppInit(opts *AppInitRequest) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "app", "init",
			"--name", opts.AppName,
			"--app-type", opts.AppType,
			"--dockerfile", opts.Dockerfile))
}

/*AppShow runs:
ecs-preview app show
	--project $p
	--app $a
	--json
*/
func (cli *CLI) AppShow(opts *AppShowRequest) (*AppShowOutput, error) {
	appJSON, appShowErr := cli.exec(
		exec.Command(cli.path, "app", "show",
			"--project", opts.ProjectName,
			"--app", opts.AppName,
			"--json"))

	if appShowErr != nil {
		return nil, appShowErr
	}

	return toAppShowOutput(appJSON)
}

/*AppDelete runs:
ecs-preview app delete
	--name $n
	--yes
*/
func (cli *CLI) AppDelete(appName string) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "app", "delete",
			"--name", appName,
			"--yes"))
}

/*AppDeploy runs:
ecs-preview app deploy
	--name $n
	--env $e
	--tag $t
*/
func (cli *CLI) AppDeploy(opts *AppDeployInput) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "app", "deploy",
			"--name", opts.AppName,
			"--env", opts.EnvName,
			"--tag", opts.ImageTag))
}

/*AppList runs:
ecs-preview app ls
	--project $p
	--json
*/
func (cli *CLI) AppList(projectName string) (*AppListOutput, error) {
	output, err := cli.exec(
		exec.Command(cli.path, "app", "ls",
			"--project", projectName,
			"--json"))
	if err != nil {
		return nil, err
	}
	return toAppListOutput(output)
}

/*AppLogs runs:
ecs-preview app logs
	--project $p
	--name $n
	--since $s
	--env $e
	--json
*/
func (cli *CLI) AppLogs(opts *AppLogsRequest) ([]AppLogsOutput, error) {
	output, err := cli.exec(
		exec.Command(cli.path, "app", "logs",
			"--project", opts.ProjectName,
			"--name", opts.AppName,
			"--since", opts.Since,
			"--env", opts.EnvName,
			"--json"))
	if err != nil {
		return nil, err
	}
	return toAppLogsOutput(output)
}

/*EnvDelete runs:
ecs-preview env delete
	--name $n
	--profile $p
	--yes
*/
func (cli *CLI) EnvDelete(envName, profile string) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "env", "delete",
			"--name", envName,
			"--profile", profile,
			"--yes"))
}

/*EnvInit runs:
ecs-preview env init
	--name $n
	--project $p
	--profile $pr
	--prod (optionally)
*/
func (cli *CLI) EnvInit(opts *EnvInitRequest) (string, error) {
	commands := []string{"env", "init",
		"--name", opts.EnvName,
		"--project", opts.ProjectName,
		"--profile", opts.Profile,
	}
	if opts.Prod {
		commands = append(commands, "--prod")
	}
	return cli.exec(exec.Command(cli.path, commands...))
}

/*EnvList runs:
ecs-preview env ls
	--project $p
	--json
*/
func (cli *CLI) EnvList(projectName string) (*EnvListOutput, error) {
	output, err := cli.exec(
		exec.Command(cli.path, "env", "ls",
			"--project", projectName,
			"--json"))
	if err != nil {
		return nil, err
	}
	return toEnvListOutput(output)
}

/*ProjectInit runs:
ecs-preview project init $p
	--domain $d (optionally)
*/
func (cli *CLI) ProjectInit(opts *ProjectInitRequest) (string, error) {
	commands := []string{"project", "init", opts.ProjectName}
	if opts.Domain != "" {
		commands = append(commands, "--domain", opts.Domain)
	}
	return cli.exec(exec.Command(cli.path, commands...))
}

/*ProjectShow runs:
ecs-preview project show
	--project $p
	--json
*/
func (cli *CLI) ProjectShow(projectName string) (*ProjectShowOutput, error) {
	output, err := cli.exec(
		exec.Command(cli.path, "project", "show",
			"--project", projectName,
			"--json"))
	if err != nil {
		return nil, err
	}
	return toProjectShowOutput(output)
}

/*ProjectList runs:
ecs-preview project ls
*/
func (cli *CLI) ProjectList() (string, error) {
	return cli.exec(exec.Command(cli.path, "project", "ls"))
}

/*ProjectDelete runs:
ecs-preview project delete
	--env-profiles $e1=$p1,$e2=$p2
	--yes
*/
func (cli *CLI) ProjectDelete(profiles map[string]string) (string, error) {
	commands := []string{"project", "delete", "--yes"}

	if len(profiles) > 0 {
		commands = append(commands, "--env-profiles")
		envProfiles := []string{}
		for env, profile := range profiles {
			envProfiles = append(envProfiles, fmt.Sprintf("%s=%s", env, profile))
		}
		commands = append(commands, strings.Join(envProfiles, ","))
	}
	return cli.exec(
		exec.Command(cli.path, commands...))
}

func (cli *CLI) exec(command *exec.Cmd) (string, error) {
	// Turn off colors
	command.Env = append(os.Environ(), "COLOR=false")
	sess, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return "", err
	}

	contents := sess.Wait(100000000).Out.Contents()
	if exitCode := sess.ExitCode(); exitCode != 0 {
		return string(contents), fmt.Errorf("received non 0 exit code")
	}

	return string(contents), nil
}
