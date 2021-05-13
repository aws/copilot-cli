// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	cmd "github.com/aws/copilot-cli/e2e/internal/command"
)

// AWS is a wrapper around aws commands.
type AWS struct{}

// VPCStackOutput is the output for VPC stack.
type VPCStackOutput struct {
	OutputKey   string
	OutputValue string
	ExportName  string
}

// NewAWS returns a wrapper around AWS commands.
func NewAWS() *AWS {
	return &AWS{}
}

/*CreateStack runs:
aws cloudformation create-stack
	--stack-name $name
	--template-body $templatePath
*/
func (a *AWS) CreateStack(name, templatePath string) error {
	command := strings.Join([]string{
		"cloudformation",
		"create-stack",
		"--stack-name", name,
		"--template-body", templatePath,
	}, " ")
	return a.exec(command)
}

/*WaitStackCreateComplete runs:
aws cloudformation wait stack-create-complete
	--stack-name $name
*/
func (a *AWS) WaitStackCreateComplete(name string) error {
	command := strings.Join([]string{
		"cloudformation",
		"wait",
		"stack-create-complete",
		"--stack-name", name,
	}, " ")
	return a.exec(command)
}

/*VPCStackOutput runs:
aws cloudformation describe-stacks --stack-name $name |
	jq -r .Stacks[0].Outputs
*/
func (a *AWS) VPCStackOutput(name string) ([]VPCStackOutput, error) {
	command := strings.Join([]string{
		"cloudformation",
		"describe-stacks",
		"--stack-name", name,
		"|",
		"jq", "-r", ".Stacks[0].Outputs",
	}, " ")
	var b bytes.Buffer
	err := a.exec(command, cmd.Stdout(&b))
	if err != nil {
		return nil, err
	}
	var outputs []VPCStackOutput
	err = json.Unmarshal(b.Bytes(), &outputs)
	if err != nil {
		return nil, err
	}
	return outputs, nil
}

/*DeleteStack runs:
aws cloudformation delete-stack --stack-name $name
*/
func (a *AWS) DeleteStack(name string) error {
	command := strings.Join([]string{
		"cloudformation",
		"delete-stack",
		"--stack-name", name,
	}, " ")
	return a.exec(command)
}

/*WaitStackDeleteComplete runs:
aws cloudformation wait stack-delete-complete
	--stack-name $name
*/
func (a *AWS) WaitStackDeleteComplete(name string) error {
	command := strings.Join([]string{
		"cloudformation",
		"wait",
		"stack-delete-complete",
		"--stack-name", name,
	}, " ")
	return a.exec(command)
}

/*CreateECRRepo runs:
aws ecr create-repository --repository-name $name |
	jq -r .repository.repositoryUri
*/
func (a *AWS) CreateECRRepo(name string) (string, error) {
	command := strings.Join([]string{
		"ecr",
		"create-repository",
		"--repository-name", name,
		"|",
		"jq", "-r", ".repository.repositoryUri",
	}, " ")
	var b bytes.Buffer
	err := a.exec(command, cmd.Stdout(&b))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

/*ECRLoginPassword runs:
aws ecr get-login-password
*/
func (a *AWS) ECRLoginPassword() (string, error) {
	command := strings.Join([]string{
		"ecr",
		"get-login-password",
	}, " ")
	var b bytes.Buffer
	err := a.exec(command, cmd.Stdout(&b))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

/*DeleteECRRepo runs:
aws ecr delete-repository
	--repository-name $name --force
*/
func (a *AWS) DeleteECRRepo(name string) error {
	command := strings.Join([]string{
		"ecr",
		"delete-repository",
		"--repository-name", name,
		"--force",
	}, " ")
	return a.exec(command)
}

func (a *AWS) exec(command string, opts ...cmd.Option) error {
	return BashExec(fmt.Sprintf("aws %s", command), opts...)
}

/*GetFileSystemSize runs:
aws efs describe-file-systems | jq -r '.FileSystems[0].SizeInBytes.Value',
which returns the size in bytes of the first filesystem returned by the call.
*/
func (a *AWS) GetFileSystemSize() (int, error) {
	command := strings.Join([]string{
		"efs",
		"describe-file-systems",
		"|",
		"jq", "-r", "'.FileSystems[0].SizeInBytes.Value'",
	}, " ")
	var b bytes.Buffer
	err := a.exec(command, cmd.Stdout(&b))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(b.String()))
}
