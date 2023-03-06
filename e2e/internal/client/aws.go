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

// IAM policy ARNs.
const (
	codeCommitPowerUserPolicyARN = "arn:aws:iam::aws:policy/AWSCodeCommitPowerUser"
)

// AWS is a wrapper around aws commands.
type AWS struct{}

// VPCStackOutput is the output for VPC stack.
type VPCStackOutput struct {
	OutputKey   string
	OutputValue string
	ExportName  string
}

// dbClusterSnapshot represents part of the response to `aws rds describe-db-cluster-snapshots`
type dbClusterSnapshot struct {
	Identifier string `json:"DBClusterSnapshotIdentifier"`
	Cluster    string `json:"DBClusterIdentifier"`
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

// CreateCodeCommitRepo creates a repository with AWS CodeCommit and returns
// the HTTP git clone url.
func (a *AWS) CreateCodeCommitRepo(name string) (cloneURL string, err error) {
	out := new(bytes.Buffer)
	args := strings.Join([]string{
		"codecommit",
		"create-repository",
		"--repository-name",
		name,
	}, " ")
	if err := a.exec(args, cmd.Stdout(out)); err != nil {
		return "", fmt.Errorf("create commit repository %s cmd: %v", name, err)
	}

	data := struct {
		RepositoryMetadata struct {
			CloneURLHTTP string `json:"cloneUrlHttp"`
		} `json:"repositoryMetadata"`
	}{}
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		return "", fmt.Errorf("unmarshal json response from create commit repository: %v", err)
	}
	return data.RepositoryMetadata.CloneURLHTTP, nil
}

// DeleteCodeCommitRepo delete a CodeCommit repository.
func (a *AWS) DeleteCodeCommitRepo(name string) error {
	args := strings.Join([]string{
		"codecommit",
		"delete-repository",
		"--repository-name",
		name,
	}, " ")
	if err := a.exec(args); err != nil {
		return fmt.Errorf("delete repository %s: %v", name, err)
	}
	return nil
}

// IAMServiceCreds represents service-specific IAM credentials.
type IAMServiceCreds struct {
	UserName     string `json:"ServiceUserName"`             // Git username.
	Password     string `json:"ServicePassword"`             // Git password.
	CredentialID string `json:"ServiceSpecificCredentialId"` // ID for the creds in order to delete them.
}

// CreateCodeCommitIAMUser creates an IAM user that can push and pull from codecommit.
// Returns the credentials needed to interact with codecommit.
func (a *AWS) CreateCodeCommitIAMUser(userName string) (*IAMServiceCreds, error) {
	if err := a.createIAMUser("/copilot/e2etests/", userName); err != nil {
		return nil, err
	}
	if err := a.attachUserPolicy(userName, codeCommitPowerUserPolicyARN); err != nil {
		return nil, err
	}
	return a.createCodeCommitCreds(userName)
}

// DeleteCodeCommitIAMUser deletes an IAM user that can access codecommit.
func (a *AWS) DeleteCodeCommitIAMUser(userName, credentialID string) error {
	if err := a.deleteServiceSpecificCreds(userName, credentialID); err != nil {
		return err
	}
	if err := a.detachUserPolicy(userName, codeCommitPowerUserPolicyARN); err != nil {
		return err
	}
	return a.deleteIAMUser(userName)
}

func (a *AWS) createIAMUser(path, userName string) error {
	args := strings.Join([]string{
		"iam",
		"create-user",
		"--path",
		path,
		"--user-name",
		userName,
	}, " ")
	if err := a.exec(args); err != nil {
		return fmt.Errorf("create IAM user under path %s and name %s: %v", path, userName, err)
	}
	return nil
}

func (a *AWS) attachUserPolicy(userName, policyARN string) error {
	args := strings.Join([]string{
		"iam",
		"attach-user-policy",
		"--user-name",
		userName,
		"--policy-arn",
		policyARN,
	}, " ")
	if err := a.exec(args); err != nil {
		return fmt.Errorf("attach policy arn %s to user %s: %v", policyARN, userName, err)
	}
	return nil
}

func (a *AWS) createCodeCommitCreds(userName string) (*IAMServiceCreds, error) {
	out := new(bytes.Buffer)
	args := strings.Join([]string{
		"iam",
		"create-service-specific-credential",
		"--user-name",
		userName,
		"--service-name",
		"codecommit.amazonaws.com",
	}, " ")
	if err := a.exec(args, cmd.Stdout(out)); err != nil {
		return nil, fmt.Errorf("create commit credentials for user %s: %v", userName, err)
	}
	data := struct {
		Creds IAMServiceCreds `json:"ServiceSpecificCredential"`
	}{}
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		return nil, fmt.Errorf("unmarshal credentials for codecommit: %v", err)
	}
	return &data.Creds, nil
}

func (a *AWS) deleteServiceSpecificCreds(userName, credentialID string) error {
	args := strings.Join([]string{
		"iam",
		"delete-service-specific-credential",
		"--user-name",
		userName,
		"--service-specific-credential-id",
		credentialID,
	}, " ")
	if err := a.exec(args); err != nil {
		return fmt.Errorf("delete service specific creds %s for user %s: %v", credentialID, userName, err)
	}
	return nil
}

func (a *AWS) detachUserPolicy(userName, policyARN string) error {
	args := strings.Join([]string{
		"iam",
		"detach-user-policy",
		"--user-name",
		userName,
		"--policy-arn",
		policyARN,
	}, " ")
	if err := a.exec(args); err != nil {
		return fmt.Errorf("detach user policy %s for user %s: %v", policyARN, userName, err)
	}
	return nil
}

func (a *AWS) deleteIAMUser(userName string) error {
	args := strings.Join([]string{
		"iam",
		"delete-user",
		"--user-name",
		userName,
	}, " ")
	if err := a.exec(args); err != nil {
		return fmt.Errorf("delete iam user %s: %v", userName, err)
	}
	return nil
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

// DeleteAllDBClusterSnapshots removes all "manual" RDS cluster snapshots to avoid running into snapshot limits.
func (a *AWS) DeleteAllDBClusterSnapshots() error {
	command := strings.Join([]string{
		"rds",
		"describe-db-cluster-snapshots",
	}, " ")
	var b bytes.Buffer
	err := a.exec(command, cmd.Stdout(&b))
	if err != nil {
		return err
	}
	var snapshotResponse struct {
		Snapshots []dbClusterSnapshot `json:"DBClusterSnapshots"`
	}
	if err = json.Unmarshal(b.Bytes(), &snapshotResponse); err != nil {
		return err
	}
	for _, s := range snapshotResponse.Snapshots {
		deleteCmd := strings.Join([]string{
			"rds",
			"delete-db-cluster-snapshot",
			"--db-cluster-snapshot-identifier",
			s.Identifier,
		}, " ")
		var err = a.exec(deleteCmd)
		if err != nil {
			return err
		}
	}
	return nil
}

type environmentFile struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}
type containerDefinition struct {
	Name             string            `json:"name"`
	EnvironmentFiles []environmentFile `json:"environmentFiles"`
}

// GetEnvFilesFromTaskDefinition describes the given task definition, then returns a map of all
// existing environment files where the keys are container names and the values are S3 locations.
func (a *AWS) GetEnvFilesFromTaskDefinition(taskDefinitionName string) (map[string]string, error) {
	command := strings.Join([]string{
		"ecs",
		"describe-task-definition",
		"--task-definition",
		taskDefinitionName,
	}, " ")
	var b bytes.Buffer
	err := a.exec(command, cmd.Stdout(&b))
	if err != nil {
		return nil, err
	}
	var containerDefinitions struct {
		TaskDefinition struct {
			ContainerDefinitions []containerDefinition `json:"containerDefinitions"`
		} `json:"taskDefinition"`
	}
	if err = json.Unmarshal(b.Bytes(), &containerDefinitions); err != nil {
		return nil, err
	}
	envFiles := make(map[string]string)
	for _, container := range containerDefinitions.TaskDefinition.ContainerDefinitions {
		if len(container.EnvironmentFiles) > 0 {
			envFiles[container.Name] = container.EnvironmentFiles[0].Value
		}
	}
	return envFiles, nil
}
