// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package patch

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

type environmentTemplateUpdateGetter interface {
	Template(stackName string) (string, error)
	UpdateEnvironmentTemplate(appName, envName, templateBody, cfnExecRoleARN string) error
}

type progress interface {
	Start(label string)
	Stop(label string)
}

// EnvironmentPatcher checks if the environment needs a patch and perform the patch when necessary.
type EnvironmentPatcher struct {
	Prog            progress
	Env             *config.Environment
	TemplatePatcher environmentTemplateUpdateGetter
}

// EnsureManagerRoleIsAllowedToUpload checks if the environment manager role has the necessary permissions to upload
// objects to bucket and patches the permissions if not.
func (p *EnvironmentPatcher) EnsureManagerRoleIsAllowedToUpload(bucket string) error {
	body, err := p.TemplatePatcher.Template(stack.NameForEnv(p.Env.App, p.Env.Name))
	if err != nil {
		return fmt.Errorf("get environment template for %q: %w", p.Env.Name, err)
	}
	ok, err := isManagerRoleAllowedToUpload(body)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return p.grantManagerRolePermissionToUpload(p.Env.App, p.Env.Name, p.Env.ExecutionRoleARN, body, s3.FormatARN(endpoints.AwsPartitionID, bucket))
}

func (p *EnvironmentPatcher) grantManagerRolePermissionToUpload(app, env, execRole, body, bucketARN string) error {
	// Detect which line number the EnvironmentManagerRole's PolicyDocument Statement is at.
	// We will add additional permissions after that line.
	type Template struct {
		Resources struct {
			ManagerRole struct {
				Properties struct {
					Policies []struct {
						Document struct {
							Statements yaml.Node `yaml:"Statement"`
						} `yaml:"PolicyDocument"`
					} `yaml:"Policies"`
				} `yaml:"Properties"`
			} `yaml:"EnvironmentManagerRole"`
		} `yaml:"Resources"`
	}

	var tpl Template
	if err := yaml.Unmarshal([]byte(body), &tpl); err != nil {
		return fmt.Errorf("unmarshal environment template to find EnvironmentManagerRole policy statement: %v", err)
	}
	if len(tpl.Resources.ManagerRole.Properties.Policies) == 0 {
		return errors.New("unable to find policies for the EnvironmentManagerRole")
	}
	// lines and columns are 1-indexed, so we have to subtract one from each.
	statementLineIndex := tpl.Resources.ManagerRole.Properties.Policies[0].Document.Statements.Line - 1
	numSpaces := tpl.Resources.ManagerRole.Properties.Policies[0].Document.Statements.Column - 1
	pad := strings.Repeat(" ", numSpaces)

	// Create the additional permissions needed with the appropriate indentation.
	permissions := fmt.Sprintf(`- Sid: PatchPutObjectsToArtifactBucket
  Effect: Allow
  Action:
    - s3:PutObject
    - s3:PutObjectAcl
  Resource:
    - %s
    - %s/*`, bucketARN, bucketARN)
	permissions = pad + strings.Replace(permissions, "\n", "\n"+pad, -1)

	// Add the new permissions to the body.
	lines := strings.Split(body, "\n")
	linesBefore := lines[:statementLineIndex]
	linesAfter := lines[statementLineIndex:]
	updatedLines := append(linesBefore, append(strings.Split(permissions, "\n"), linesAfter...)...)
	updatedBody := strings.Join(updatedLines, "\n")

	// Update the Environment template with the new content.
	// CloudFormation is the only entity that's allowed to update the EnvManagerRole so we have to go through this route.
	// See #3556.
	var errEmptyChangeSet *cloudformation.ErrChangeSetEmpty
	p.Prog.Start("Update the environment's manager role with permission to upload artifacts to S3")
	err := p.TemplatePatcher.UpdateEnvironmentTemplate(app, env, updatedBody, execRole)
	if err != nil && !errors.As(err, &errEmptyChangeSet) {
		p.Prog.Stop(log.Serrorln("Unable to update the environment's manager role with upload artifacts permission"))
		return fmt.Errorf("update environment template with PutObject permissions: %v", err)
	}
	p.Prog.Stop(log.Ssuccessln("Updated the environment's manager role with permissions to upload artifacts to S3"))
	return nil
}

func isManagerRoleAllowedToUpload(body string) (bool, error) {
	type Template struct {
		Metadata struct {
			Version string `yaml:"Version"`
		} `yaml:"Metadata"`
	}
	var tpl Template
	if err := yaml.Unmarshal([]byte(body), &tpl); err != nil {
		return false, fmt.Errorf("unmarshal environment template to detect Metadata.Version: %v", err)
	}
	if tpl.Metadata.Version == version.EnvTemplateBootstrap {
		// "bootstrap" version is introduced after v1.9.0. The environment manager roles must have had the permissions.
		return true, nil
	}
	if !semver.IsValid(tpl.Metadata.Version) { // The template doesn't contain a version.
		return false, nil
	}
	if semver.Compare(tpl.Metadata.Version, "v1.9.0") < 0 {
		// The permissions to grant the EnvManagerRole to upload artifacts was granted with template v1.9.0.
		return false, nil
	}
	return true, nil
}
