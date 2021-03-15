// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

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

// CLI is a wrapper around os.execs.
type CLI struct {
	path string
}

// AppInitRequest contains the parameters for calling copilot app init.
type AppInitRequest struct {
	AppName string
	Domain  string
	Tags    map[string]string
}

// InitRequest contains the parameters for calling copilot init.
type InitRequest struct {
	AppName      string
	WorkloadName string
	Deploy       bool
	ImageTag     string
	Dockerfile   string
	WorkloadType string
	SvcPort      string
	Schedule     string
}

// EnvInitRequest contains the parameters for calling copilot env init.
type EnvInitRequest struct {
	AppName       string
	EnvName       string
	Profile       string
	Prod          bool
	CustomizedEnv bool
	VPCImport     EnvInitRequestVPCImport
	VPCConfig     EnvInitRequestVPCConfig
}

// EnvInitRequestVPCImport contains the parameters for configuring VPC import when
// calling copilot env init.
type EnvInitRequestVPCImport struct {
	ID               string
	PublicSubnetIDs  string
	PrivateSubnetIDs string
}

// IsSet returns true if all fields are set.
func (e EnvInitRequestVPCImport) IsSet() bool {
	return e.ID != "" && e.PublicSubnetIDs != "" && e.PrivateSubnetIDs != ""
}

// EnvInitRequestVPCConfig contains the parameters for configuring VPC config when
// calling copilot env init.
type EnvInitRequestVPCConfig struct {
	CIDR               string
	PublicSubnetCIDRs  string
	PrivateSubnetCIDRs string
}

// EnvShowRequest contains the parameters for calling copilot env show.
type EnvShowRequest struct {
	AppName string
	EnvName string
}

// SvcInitRequest contains the parameters for calling copilot svc init.
type SvcInitRequest struct {
	Name       string
	SvcType    string
	Dockerfile string
	Image      string
	SvcPort    string
}

// SvcShowRequest contains the parameters for calling copilot svc show.
type SvcShowRequest struct {
	Name    string
	AppName string
}

// SvcStatusRequest contains the parameters for calling copilot svc status.
type SvcStatusRequest struct {
	Name    string
	AppName string
	EnvName string
}

// SvcExecRequest contains the parameters for calling copilot svc exec.
type SvcExecRequest struct {
	Name      string
	AppName   string
	Command   string
	TaskID    string
	Container string
	EnvName   string
}

// SvcLogsRequest contains the parameters for calling copilot svc logs.
type SvcLogsRequest struct {
	AppName string
	EnvName string
	Name    string
	Since   string
}

// SvcDeployInput contains the parameters for calling copilot svc deploy.
type SvcDeployInput struct {
	Name     string
	EnvName  string
	ImageTag string
}

// TaskRunInput contains the parameters for calling copilot task run.
type TaskRunInput struct {
	AppName string

	GroupName string

	Image      string
	Dockerfile string

	Subnets        []string
	SecurityGroups []string
	Env            string

	Command string
	EnvVars string

	Default bool
	Follow  bool
}

// TaskExecRequest contains the parameters for calling copilot task exec.
type TaskExecRequest struct {
	Name    string
	AppName string
	Command string
	EnvName string
}

// TaskDeleteInput contains the parameters for calling copilot task delete.
type TaskDeleteInput struct {
	App     string
	Env     string
	Name    string
	Default bool
}

// JobInitInput contains the parameters for calling copilot job init.
type JobInitInput struct {
	Name       string
	Dockerfile string
	Schedule   string
	Retries    string
	Timeout    string
}

// JobDeployInput contains the parameters for calling copilot job deploy.
type JobDeployInput struct {
	Name     string
	EnvName  string
	ImageTag string
}

// PackageInput contains the parameters for calling copilot job package.
type PackageInput struct {
	AppName string
	Name    string
	Env     string
	Dir     string
	Tag     string
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
	--deploy (optionally)
	--schedule $schedule (optionally)
	--port $port (optionally)
*/
func (cli *CLI) Init(opts *InitRequest) (string, error) {
	var deployOption string
	var scheduleOption string
	var portOption string

	if opts.Deploy {
		deployOption = "--deploy"
	}
	if opts.Schedule != "" {
		scheduleOption = "--schedule"
	}
	if opts.SvcPort != "" {
		portOption = "--port"
	}

	return cli.exec(
		exec.Command(cli.path, "init",
			"--app", opts.AppName,
			"--name", opts.WorkloadName,
			"--type", opts.WorkloadType,
			"--tag", opts.ImageTag,
			"--dockerfile", opts.Dockerfile,
			deployOption,
			scheduleOption, opts.Schedule,
			portOption, opts.SvcPort))
}

/*SvcInit runs:
copilot svc init
	--name $n
	--svc-type $t
	--port $port
*/
func (cli *CLI) SvcInit(opts *SvcInitRequest) (string, error) {
	args := []string{
		"svc",
		"init",
		"--name", opts.Name,
		"--svc-type", opts.SvcType,
	}
	// Apply optional flags only if a value is provided.
	if opts.SvcPort != "" {
		args = append(args, "--port", opts.SvcPort)
	}
	if opts.Dockerfile != "" {
		args = append(args, "--dockerfile", opts.Dockerfile)
	}
	if opts.Image != "" {
		args = append(args, "--image", opts.Image)
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

/*SvcStatus runs:
copilot svc status
	--app $p
	--env $e
	--name $n
	--json
*/
func (cli *CLI) SvcStatus(opts *SvcStatusRequest) (*SvcStatusOutput, error) {
	svcJSON, svcStatusErr := cli.exec(
		exec.Command(cli.path, "svc", "status",
			"--app", opts.AppName,
			"--name", opts.Name,
			"--env", opts.EnvName,
			"--json"))

	if svcStatusErr != nil {
		return nil, svcStatusErr
	}

	return toSvcStatusOutput(svcJSON)
}

/*SvcExec runs:
copilot svc exec
	--app $p
	--env $e
	--name $n
	--command $cmd
	--container $ctnr
	--task-id $td
	--yes
*/
func (cli *CLI) SvcExec(opts *SvcExecRequest) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "svc", "exec",
			"--app", opts.AppName,
			"--name", opts.Name,
			"--env", opts.EnvName,
			"--command", opts.Command,
			"--container", opts.Container,
			"--task-id", opts.TaskID,
			"--yes"))
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
	--yes
*/
func (cli *CLI) EnvDelete(envName string) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "env", "delete",
			"--name", envName,
			"--yes"))
}

/*EnvInit runs:
copilot env init
	--name $n
	--app $a
	--profile $pr
	--prod (optional)
	--default-config (optional)
	--import-private-subnets (optional)
	--import-public-subnets (optional)
	--import-vpc-id (optional)
	--override-private-cidrs (optional)
	--override-public-cidrs (optional)
	--override-vpc-cidr (optional)
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
	if !opts.CustomizedEnv {
		commands = append(commands, "--default-config")
	}
	if (opts.VPCImport != EnvInitRequestVPCImport{}) {
		commands = append(commands, "--import-vpc-id", opts.VPCImport.ID, "--import-public-subnets",
			opts.VPCImport.PublicSubnetIDs, "--import-private-subnets", opts.VPCImport.PrivateSubnetIDs)
	}
	if (opts.VPCConfig != EnvInitRequestVPCConfig{}) {
		commands = append(commands, "--override-vpc-cidr", opts.VPCConfig.CIDR, "--override-public-cidrs",
			opts.VPCConfig.PublicSubnetCIDRs, "--override-private-cidrs", opts.VPCConfig.PrivateSubnetCIDRs)
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
	--resource-tags $k1=$v1,$k2=$k2 (optionally)
*/
func (cli *CLI) AppInit(opts *AppInitRequest) (string, error) {
	commands := []string{"app", "init", opts.AppName}
	if opts.Domain != "" {
		commands = append(commands, "--domain", opts.Domain)
	}

	if len(opts.Tags) > 0 {
		commands = append(commands, "--resource-tags")
		tags := []string{}
		for key, val := range opts.Tags {
			tags = append(tags, fmt.Sprintf("%s=%s", key, val))
		}
		commands = append(commands, strings.Join(tags, ","))
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
copilot app delete --yes
*/
func (cli *CLI) AppDelete() (string, error) {
	commands := []string{"app", "delete", "--yes"}

	return cli.exec(
		exec.Command(cli.path, commands...))
}

/*TaskRun runs:
copilot task run
	-n $t
	--dockerfile $d
	--app $a (optionally)
	--env $e (optionally)
	--command $c (optionally)
	--env-vars $e1=$v1,$e2=$v2 (optionally)
	--default (optionally)
	--follow (optionally)
*/
func (cli *CLI) TaskRun(input *TaskRunInput) (string, error) {
	commands := []string{"task", "run", "-n", input.GroupName, "--dockerfile", input.Dockerfile}
	if input.Image != "" {
		commands = append(commands, "--image", input.Image)
	}
	if input.AppName != "" {
		commands = append(commands, "--app", input.AppName)
	}
	if input.Env != "" {
		commands = append(commands, "--env", input.Env)
	}
	if input.Command != "" {
		commands = append(commands, "--command", input.Command)
	}
	if input.EnvVars != "" {
		commands = append(commands, "--env-vars", input.EnvVars)
	}
	if input.Default {
		commands = append(commands, "--default")
	}
	if input.Follow {
		commands = append(commands, "--follow")
	}
	return cli.exec(exec.Command(cli.path, commands...))
}

/*TaskExec runs:
copilot task exec
	--app $p
	--env $e
	--name $n
	--command $cmd
*/
func (cli *CLI) TaskExec(opts *TaskExecRequest) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "task", "exec",
			"--app", opts.AppName,
			"--name", opts.Name,
			"--env", opts.EnvName,
			"--command", opts.Command))
}

/*TaskDelete runs:
copilot task delete
	--name $n
	--yes
	--default (optionally)
	--app $a (optionally)
	--env $e (optionally)
*/
func (cli *CLI) TaskDelete(opts *TaskDeleteInput) (string, error) {
	args := []string{
		"task",
		"delete",
		"--name", opts.Name,
		"--yes",
	}
	if opts.App != "" {
		args = append(args, "--app", opts.App)
	}
	if opts.Env != "" {
		args = append(args, "--env", opts.Env)
	}
	if opts.Default {
		args = append(args, "--default")
	}
	return cli.exec(
		exec.Command(cli.path, args...),
	)
}

/*JobInit runs:
copilot job init
	--name $n
	--dockerfile $d
	--schedule $sched
	--retries $r
	--timeout $o
*/
func (cli *CLI) JobInit(opts *JobInitInput) (string, error) {
	args := []string{
		"job",
		"init",
		"--name", opts.Name,
		"--dockerfile", opts.Dockerfile,
		"--schedule", opts.Schedule,
	}
	// Apply optional flags only if a value is provided.
	if opts.Retries != "" {
		args = append(args, "--retries", opts.Retries)
	}
	if opts.Timeout != "" {
		args = append(args, "--timeout", opts.Timeout)
	}
	return cli.exec(
		exec.Command(cli.path, args...))
}

/*JobDeploy runs:
copilot job deploy
	--name $n
	--env $e
	--tag $t
*/
func (cli *CLI) JobDeploy(opts *JobDeployInput) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "job", "deploy",
			"--name", opts.Name,
			"--env", opts.EnvName,
			"--tag", opts.ImageTag))
}

/*JobDelete runs:
copilot job delete
	--name $n
	--yes
*/
func (cli *CLI) JobDelete(jobName string) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "job", "delete",
			"--name", jobName,
			"--yes"))
}

/*JobList runs:
copilot job ls
	--json?
	--local?
*/
func (cli *CLI) JobList(appName string) (*JobListOutput, error) {
	output, err := cli.exec(
		exec.Command(cli.path, "job", "ls",
			"--app", appName,
			"--json"))
	if err != nil {
		return nil, err
	}
	return toJobListOutput(output)
}

/*JobPackage runs:
copilot job package
	--output-dir $dir
	--name $name
	--env $env
	--app $appname
	--tag $tag
*/
func (cli *CLI) JobPackage(opts *PackageInput) error {
	args := []string{
		"job",
		"package",
		"--name", opts.Name,
		"--env", opts.Env,
		"--app", opts.AppName,
		"--output-dir", opts.Dir,
		"--tag", opts.Tag,
	}

	_, err := cli.exec(exec.Command(cli.path, args...))
	return err
}

/*SvcPackage runs:
copilot svc package
	--output-dir $dir
	--name $name
	--env $env
	--app $appname
*/
func (cli *CLI) SvcPackage(opts *PackageInput) error {
	args := []string{
		"svc",
		"package",
		"--name", opts.Name,
		"--env", opts.Env,
		"--app", opts.AppName,
		"--output-dir", opts.Dir,
		"--tag", opts.Tag,
	}
	_, err := cli.exec(exec.Command(cli.path, args...))
	return err
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
