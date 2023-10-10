// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega/gexec"
)

// CLI is a wrapper around os.execs.
type CLI struct {
	path       string
	workingDir string
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
	EnvName      string
	Deploy       bool
	ImageTag     string
	Dockerfile   string
	WorkloadType string
	SvcPort      string
	Schedule     string
}

// EnvInitRequest contains the parameters for calling copilot env init.
type EnvInitRequest struct {
	AppName           string
	EnvName           string
	Profile           string
	CustomizedEnv     bool
	VPCImport         EnvInitRequestVPCImport
	VPCConfig         EnvInitRequestVPCConfig
	CertificateImport string
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
	AZs                string
	PublicSubnetCIDRs  string
	PrivateSubnetCIDRs string
}

// EnvDeployRequest contains the parameters for calling copilot env deploy.
type EnvDeployRequest struct {
	AppName string
	Name    string
}

// EnvShowRequest contains the parameters for calling copilot env show.
type EnvShowRequest struct {
	AppName string
	EnvName string
}

// SvcInitRequest contains the parameters for calling copilot svc init.
type SvcInitRequest struct {
	Name               string
	SvcType            string
	Dockerfile         string
	Image              string
	SvcPort            string
	TopicSubscriptions []string
	IngressType        string
}

// SvcShowRequest contains the parameters for calling copilot svc show.
type SvcShowRequest struct {
	Name      string
	AppName   string
	Resources bool
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

// SvcPauseRequest contains the parameters for calling copilot svc logs.
type SvcPauseRequest struct {
	AppName string
	EnvName string
	Name    string
}

// SvcResumeRequest contains the parameters for calling copilot svc logs.
type SvcResumeRequest struct {
	AppName string
	EnvName string
	Name    string
}

// StorageInitRequest contains the parameters for calling copilot storage init.
type StorageInitRequest struct {
	StorageName  string
	StorageType  string
	WorkloadName string
	Lifecycle    string

	RDSEngine     string
	InitialDBName string
}

// SvcDeployInput contains the parameters for calling copilot svc deploy.
type SvcDeployInput struct {
	Name     string
	EnvName  string
	ImageTag string
	Force    bool
}

// DeployRequest contains parameters for calling copilot deploy --all.
type DeployRequest struct {
	All     bool
	EnvName string
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

// PipelineInitInput contains the parameters for calling copilot pipeline init.
type PipelineInitInput struct {
	Name         string
	URL          string
	GitBranch    string
	Environments []string
	Type         string
}

// PipelineDeployInput contains the parameters for calling copilot pipeline deploy.
type PipelineDeployInput struct {
	Name string
}

// PipelineShowInput contains the parameters for calling copilot pipeline show.
type PipelineShowInput struct {
	Name string
}

// PipelineStatusInput contains the parameters for calling copilot pipeline status.
type PipelineStatusInput struct {
	Name string
}

// NewCLI returns a wrapper around CLI.
func NewCLI() (*CLI, error) {
	// These tests should be run in a dockerfile so that
	// your file system and docker image repo isn't polluted
	// with test data and files. Since this is going to run
	// from Docker, the binary will be located in the root bin.
	cliPath := filepath.Join("/", "bin", "copilot")
	if os.Getenv("DRYRUN") == "true" {
		cliPath = filepath.Join("..", "..", "bin", "local", "copilot")
	}
	if _, err := os.Stat(cliPath); err != nil {
		return nil, err
	}

	return &CLI{
		path: cliPath,
	}, nil
}

// NewCLIWithDir returns the Copilot CLI such that the commands are run in the specified
// working directory.
func NewCLIWithDir(workingDir string) (*CLI, error) {
	cli, err := NewCLI()
	if err != nil {
		return nil, err
	}
	cli.workingDir = workingDir
	return cli, nil
}

/*
Help runs
copilot --help
*/
func (cli *CLI) Help() (string, error) {
	return cli.exec(exec.Command(cli.path, "--help"))
}

/*
Version runs:
copilot --version
*/
func (cli *CLI) Version() (string, error) {
	return cli.exec(exec.Command(cli.path, "--version"))
}

/*
Init runs:
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
			"--env", opts.EnvName,
			"--type", opts.WorkloadType,
			"--tag", opts.ImageTag,
			"--dockerfile", opts.Dockerfile,
			deployOption,
			scheduleOption, opts.Schedule,
			portOption, opts.SvcPort))
}

/*
SvcInit runs:
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
	if len(opts.TopicSubscriptions) > 0 {
		args = append(args, "--subscribe-topics", strings.Join(opts.TopicSubscriptions, ","))
	}
	if opts.IngressType != "" {
		args = append(args, "--ingress-type", opts.IngressType)
	}
	return cli.exec(
		exec.Command(cli.path, args...))
}

/*
SvcShow runs:
copilot svc show

	--app $p
	--name $n
	--json
*/
func (cli *CLI) SvcShow(opts *SvcShowRequest) (*SvcShowOutput, error) {
	args := []string{
		"svc", "show",
		"--app", opts.AppName,
		"--name", opts.Name,
		"--json",
	}

	if opts.Resources {
		args = append(args, "--resources")
	}

	svcJSON, svcShowErr := cli.exec(
		exec.Command(cli.path, args...))

	if svcShowErr != nil {
		return nil, svcShowErr
	}

	return toSvcShowOutput(svcJSON)
}

/*
SvcStatus runs:
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

/*
SvcExec runs:
copilot svc exec

	--app $p
	--env $e
	--name $n
	--command $cmd
	--container $ctnr
	--task-id $td
	--yes=false
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
			"--yes=false"))
}

/*
SvcDelete runs:
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

/*
SvcDeploy runs:
copilot svc deploy

	--name $n
	--env $e
	--tag $t
*/
func (cli *CLI) SvcDeploy(opts *SvcDeployInput) (string, error) {
	arguments := []string{
		"svc", "deploy",
		"--name", opts.Name,
		"--env", opts.EnvName,
		"--tag", opts.ImageTag}
	if opts.Force {
		arguments = append(arguments, "--force")
	}
	return cli.exec(
		exec.Command(cli.path, arguments...))
}

/*
Deploy runs:
copilot deploy

	--env $p
	--all

It does not initialize any workloads or environments, simply deploys all initialized services
and jobs in the workspace.
*/
func (cli *CLI) Deploy(opts *DeployRequest) (string, error) {
	arguments := []string{
		"deploy",
		"--env", opts.EnvName,
	}
	if opts.All {
		arguments = append(arguments, "--all")
	}
	return cli.exec(
		exec.Command(cli.path, arguments...),
	)
}

/*
SvcList runs:
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

/*
SvcLogs runs:
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

/*
SvcPause runs:
copilot svc pause

	--app $p
	--name $n
	--env $e
*/
func (cli *CLI) SvcPause(opts *SvcPauseRequest) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "svc", "pause",
			"--app", opts.AppName,
			"--name", opts.Name,
			"--env", opts.EnvName,
			"--yes"))
}

/*
SvcResume runs:
copilot svc pause

	--app $p
	--name $n
	--env $e
*/
func (cli *CLI) SvcResume(opts *SvcResumeRequest) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "svc", "resume",
			"--app", opts.AppName,
			"--name", opts.Name,
			"--env", opts.EnvName))
}

/*
StorageInit runs:
copilot storage init

		--name $n
		--storage-type $t
		--workload $w
		--engine $e
	  --initial-db $d
*/
func (cli *CLI) StorageInit(opts *StorageInitRequest) (string, error) {
	arguments := []string{
		"storage", "init",
		"--name", opts.StorageName,
		"--storage-type", opts.StorageType,
		"--workload", opts.WorkloadName,
		"--lifecycle", opts.Lifecycle,
	}

	if opts.RDSEngine != "" {
		arguments = append(arguments, "--engine", opts.RDSEngine)
	}

	if opts.InitialDBName != "" {
		arguments = append(arguments, "--initial-db", opts.InitialDBName)
	}

	return cli.exec(
		exec.Command(cli.path, arguments...))
}

/*
EnvDelete runs:
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

/*
EnvInit runs:
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
	if opts.CertificateImport != "" {
		commands = append(commands, "--import-cert-arns", opts.CertificateImport)
	}
	if !opts.CustomizedEnv {
		commands = append(commands, "--default-config")
	}
	if (opts.VPCImport != EnvInitRequestVPCImport{}) {
		commands = append(commands, "--import-vpc-id", opts.VPCImport.ID, "--import-public-subnets",
			opts.VPCImport.PublicSubnetIDs, "--import-private-subnets", opts.VPCImport.PrivateSubnetIDs)
	}
	if (opts.VPCConfig != EnvInitRequestVPCConfig{}) {
		commands = append(commands, "--override-vpc-cidr", opts.VPCConfig.CIDR,
			"--override-az-names", opts.VPCConfig.AZs,
			"--override-public-cidrs", opts.VPCConfig.PublicSubnetCIDRs,
			"--override-private-cidrs", opts.VPCConfig.PrivateSubnetCIDRs)
	}
	return cli.exec(exec.Command(cli.path, commands...))
}

/*
EnvDeploy runs:
copilot env deploy

	--name $n
	--app $a
*/
func (cli *CLI) EnvDeploy(opts *EnvDeployRequest) (string, error) {
	commands := []string{"env", "deploy",
		"--name", opts.Name,
		"--app", opts.AppName,
	}
	return cli.exec(exec.Command(cli.path, commands...))
}

/*
EnvShow runs:
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
			"--json", "--resources"))

	if envShowErr != nil {
		return nil, envShowErr
	}
	return toEnvShowOutput(envJSON)
}

/*
EnvList runs:
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

/*
AppInit runs:
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

/*
AppShow runs:
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

// PipelineInit runs:
//
//	copilot pipeline init
//	--name $n
//	--url $t
//	--git-branch $b
//	--environments $e[0],$e[1],...
func (cli *CLI) PipelineInit(opts PipelineInitInput) (string, error) {
	args := []string{
		"pipeline",
		"init",
		"--name", opts.Name,
		"--url", opts.URL,
		"--git-branch", opts.GitBranch,
		"--environments", strings.Join(opts.Environments, ","),
		"--pipeline-type", opts.Type,
	}

	return cli.exec(exec.Command(cli.path, args...))
}

// PipelineDeploy runs:
//
//	copilot pipeline deploy
//	--name $n
//	--yes
func (cli *CLI) PipelineDeploy(opts PipelineDeployInput) (string, error) {
	args := []string{
		"pipeline",
		"deploy",
		"--name", opts.Name,
		"--yes",
	}

	return cli.exec(exec.Command(cli.path, args...))
}

// PipelineShow runs:
//
//	copilot pipeline show
//	--name $n
//	--json
func (cli *CLI) PipelineShow(opts PipelineShowInput) (PipelineShowOutput, error) {
	args := []string{
		"pipeline",
		"show",
		"--name", opts.Name,
		"--json",
	}

	text, err := cli.exec(exec.Command(cli.path, args...))
	if err != nil {
		return PipelineShowOutput{}, err
	}

	var out PipelineShowOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return PipelineShowOutput{}, err
	}
	return out, nil
}

// PipelineStatus runs:
//
//	copilot pipeline show
//	--name $n
//	--json
func (cli *CLI) PipelineStatus(opts PipelineStatusInput) (PipelineStatusOutput, error) {
	args := []string{
		"pipeline",
		"status",
		"--name", opts.Name,
		"--json",
	}

	text, err := cli.exec(exec.Command(cli.path, args...))
	if err != nil {
		return PipelineStatusOutput{}, err
	}

	var out PipelineStatusOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return PipelineStatusOutput{}, err
	}
	return out, nil
}

/*
AppList runs:
copilot app ls
*/
func (cli *CLI) AppList() (string, error) {
	return cli.exec(exec.Command(cli.path, "app", "ls"))
}

/*
AppDelete runs:
copilot app delete --yes
*/
func (cli *CLI) AppDelete() (string, error) {
	commands := []string{"app", "delete", "--yes"}

	return cli.exec(
		exec.Command(cli.path, commands...))
}

/*
TaskRun runs:
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

/*
TaskExec runs:
copilot task exec

	--app $p
	--env $e
	--name $n
	--command $cmd
	--yes=false
*/
func (cli *CLI) TaskExec(opts *TaskExecRequest) (string, error) {
	return cli.exec(
		exec.Command(cli.path, "task", "exec",
			"--app", opts.AppName,
			"--name", opts.Name,
			"--env", opts.EnvName,
			"--command", opts.Command,
			"--yes=false"))
}

/*
TaskDelete runs:
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

/*
JobInit runs:
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

/*
JobDeploy runs:
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

/*
JobDelete runs:
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

/*
JobList runs:
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

/*
JobPackage runs:
copilot job package

	--output-dir $dir
	--name $name
	--env $env
	--app $appname
	--tag $tag
*/
func (cli *CLI) JobPackage(opts *PackageInput) (string, error) {
	args := []string{
		"job",
		"package",
		"--name", opts.Name,
		"--env", opts.Env,
		"--app", opts.AppName,
		"--output-dir", opts.Dir,
		"--tag", opts.Tag,
	}

	if opts.Dir != "" {
		args = append(args, "--output-dir", opts.Dir)
	}

	if opts.Tag != "" {
		args = append(args, "--tag", opts.Tag)
	}

	return cli.exec(exec.Command(cli.path, args...))
}

/*
SvcPackage runs:
copilot svc package

	--output-dir $dir
	--name $name
	--env $env
	--app $appname
*/
func (cli *CLI) SvcPackage(opts *PackageInput) (string, error) {
	args := []string{
		"svc",
		"package",
		"--name", opts.Name,
		"--env", opts.Env,
		"--app", opts.AppName,
	}

	if opts.Dir != "" {
		args = append(args, "--output-dir", opts.Dir)
	}

	if opts.Tag != "" {
		args = append(args, "--tag", opts.Tag)
	}

	return cli.exec(exec.Command(cli.path, args...))
}

func (cli *CLI) exec(command *exec.Cmd) (string, error) {
	// Turn off colors
	command.Env = append(os.Environ(), "COLOR=false", "CI=true")
	command.Dir = cli.workingDir
	sess, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return "", err
	}

	contents := sess.Wait(100000000).Out.Contents()
	if exitCode := sess.ExitCode(); exitCode != 0 {
		return string(sess.Err.Contents()), fmt.Errorf("received non 0 exit code")
	}

	return string(contents), nil
}
