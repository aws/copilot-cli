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

// AppInitRequest contains the parameters for calling copilot app init
type AppInitRequest struct {
	AppName string
	Domain  string
}

// InitRequest contains the parameters for calling copilot init
type InitRequest struct {
	AppName    string
	SvcName    string
	Deploy     bool
	ImageTag   string
	Dockerfile string
	SvcType    string
	SvcPort    string
}

// EnvInitRequest contains the parameters for calling copilot env init
type EnvInitRequest struct {
	AppName string
	EnvName string
	Profile string
	Prod    bool
}

// EnvShowRequest contains the parameters for calling copilot env show
type EnvShowRequest struct {
	AppName string
	EnvName string
}

// SvcInitRequest contains the parameters for calling copilot svc init
type SvcInitRequest struct {
	Name       string
	SvcType    string
	Dockerfile string
	SvcPort    string
}

// SvcShowRequest contains the parameters for calling copilot svc show
type SvcShowRequest struct {
	Name    string
	AppName string
}

// SvcLogsRequest contains the parameters for calling copilot svc logs
type SvcLogsRequest struct {
	AppName string
	EnvName string
	Name    string
	Since   string
}

// SvcDeployInput contains the parameters for calling copilot svc deploy
type SvcDeployInput struct {
	Name     string
	EnvName  string
	ImageTag string
}

// NewCLI returns a wrapper around CLI
func NewCLI() (*CLI, error) {
	// These tests should be run in a dockerfile so that
	// your file system and docker image repo isn't polluted
	// with test data and files. Since this is going to run
	// from Docker, the binary will localted in the root bin.
	cliPath := filepath.Join("/", "bin", "copilot")
	if _, err := os.Stat(cliPath); err != nil {
		return nil, err
	}

	return &CLI{
		path: cliPath,
	}, nil
}

/*Help runs
copilot --help
*/
func (cli *CLI) Help() (string, error) {
	return cli.exec(exec.Command(cli.path, "--help"))
}

/*Version runs:
copilot --version
*/
func (cli *CLI) Version() (string, error) {
	return cli.exec(exec.Command(cli.path, "--version"))
}

/*Init runs:
copilot init
	--app $p
	--svc $s
	--svc-type $type
	--tag $t
	--dockerfile $d
	--port $port
	--deploy (optionally)
*/
func (cli *CLI) Init(opts *InitRequest) (string, error) {
	var deployOption string

	if opts.Deploy {
		deployOption = "--deploy"
	}

	return cli.exec(
		exec.Command(cli.path, "init",
			"--app", opts.AppName,
			"--svc", opts.SvcName,
			"--svc-type", opts.SvcType,
			"--tag", opts.ImageTag,
			"--dockerfile", opts.Dockerfile,
			"--port", opts.SvcPort,
			deployOption))
}

/*SvcInit runs:
copilot svc init
	--name $n
	--svc-type $t
	--dockerfile $d
	--port $port
*/
func (cli *CLI) SvcInit(opts *SvcInitRequest) (string, error) {
	args := []string{
		"svc",
		"init",
		"--name", opts.Name,
		"--svc-type", opts.SvcType,
		"--dockerfile", opts.Dockerfile,
	}
	// Apply optional flags only if a value is provided.
	if opts.SvcPort != "" {
		args = append(args, "--port", opts.SvcPort)
	}
	return cli.exec(
		exec.Command(cli.path, args...))
}

/*SvcShow runs:
copilot svc show
	--app $p
	--name $n
	--json
*/
func (cli *CLI) SvcShow(opts *SvcShowRequest) (*SvcShowOutput, error) {
	svcJSON, svcShowErr := cli.exec(
		exec.Command(cli.path, "svc", "show",
			"--app", opts.AppName,
			"--name", opts.Name,
			"--json"))

	if svcShowErr != nil {
		return nil, svcShowErr
	}

	return toSvcShowOutput(svcJSON)
}

/*SvcDelete runs:
copilot svc delete
	--name $n
	--yes
*/
func (cli *CLI) SvcDelete(serviceName string) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "svc", "delete",
			"--name", serviceName,
			"--yes"))
}

/*SvcDeploy runs:
copilot svc deploy
	--name $n
	--env $e
	--tag $t
*/
func (cli *CLI) SvcDeploy(opts *SvcDeployInput) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "svc", "deploy",
			"--name", opts.Name,
			"--env", opts.EnvName,
			"--tag", opts.ImageTag))
}

/*SvcList runs:
copilot svc ls
	--app $p
	--json
*/
func (cli *CLI) SvcList(appName string) (*SvcListOutput, error) {
	output, err := cli.exec(
		exec.Command(cli.path, "svc", "ls",
			"--app", appName,
			"--json"))
	if err != nil {
		return nil, err
	}
	return toSvcListOutput(output)
}

/*SvcLogs runs:
copilot svc logs
	--app $p
	--name $n
	--since $s
	--env $e
	--json
*/
func (cli *CLI) SvcLogs(opts *SvcLogsRequest) ([]SvcLogsOutput, error) {
	output, err := cli.exec(
		exec.Command(cli.path, "svc", "logs",
			"--app", opts.AppName,
			"--name", opts.Name,
			"--since", opts.Since,
			"--env", opts.EnvName,
			"--json"))
	if err != nil {
		return nil, err
	}
	return toSvcLogsOutput(output)
}

/*EnvDelete runs:
copilot env delete
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
copilot env init
	--name $n
	--app $a
	--profile $pr
	--prod (optionally)
*/
func (cli *CLI) EnvInit(opts *EnvInitRequest) (string, error) {
	commands := []string{"env", "init",
		"--name", opts.EnvName,
		"--app", opts.AppName,
		"--profile", opts.Profile,
	}
	if opts.Prod {
		commands = append(commands, "--prod")
	}
	return cli.exec(exec.Command(cli.path, commands...))
}

/*EnvShow runs:
copilot env show
	--app $a
	--name $n
	--json
*/
func (cli *CLI) EnvShow(opts *EnvShowRequest) (*EnvShowOutput, error) {
	envJSON, envShowErr := cli.exec(
		exec.Command(cli.path, "env", "show",
			"--app", opts.AppName,
			"--name", opts.EnvName,
			"--json"))

	if envShowErr != nil {
		return nil, envShowErr
	}
	return toEnvShowOutput(envJSON)
}

/*EnvList runs:
copilot env ls
	--app $a
	--json
*/
func (cli *CLI) EnvList(appName string) (*EnvListOutput, error) {
	output, err := cli.exec(
		exec.Command(cli.path, "env", "ls",
			"--app", appName,
			"--json"))
	if err != nil {
		return nil, err
	}
	return toEnvListOutput(output)
}

/*AppInit runs:
copilot app init $a
	--domain $d (optionally)
*/
func (cli *CLI) AppInit(opts *AppInitRequest) (string, error) {
	commands := []string{"app", "init", opts.AppName}
	if opts.Domain != "" {
		commands = append(commands, "--domain", opts.Domain)
	}
	return cli.exec(exec.Command(cli.path, commands...))
}

/*AppShow runs:
copilot app show
	--name $n
	--json
*/
func (cli *CLI) AppShow(appName string) (*AppShowOutput, error) {
	output, err := cli.exec(
		exec.Command(cli.path, "app", "show",
			"--name", appName,
			"--json"))
	if err != nil {
		return nil, err
	}
	return toAppShowOutput(output)
}

/*AppList runs:
copilot app ls
*/
func (cli *CLI) AppList() (string, error) {
	return cli.exec(exec.Command(cli.path, "app", "ls"))
}

/*AppDelete runs:
copilot app delete
	--env-profiles $e1=$p1,$e2=$p2
	--yes
*/
func (cli *CLI) AppDelete(profiles map[string]string) (string, error) {
	commands := []string{"app", "delete", "--yes"}

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
