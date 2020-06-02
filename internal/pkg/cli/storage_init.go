// Copyright Amazon.com, Inc or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	dynamoDBStorageType = "DynamoDB"
	s3StorageType       = "S3"
)

var storageTypes = []string{
	dynamoDBStorageType,
	s3StorageType,
}

var (
	fmtStorageInitTypePrompt = "What " + color.Emphasize("type") + " of storage would you like to associate with %s?"
	storageInitTypeHelp      = `The type of storage you'd like to add to your service. 
DynamoDB is a key-value and document database that delivers single-digit millisecond performance at any scale.
S3 is a web object store built to store and retrieve any amount of data from anywhere on the Internet.`
	fmtStorageInitNamePrompt = "What would you like to " + color.Emphasize("name") + " this %s?"
	storageInitNameHelp      = "The name of this storage resource. You can use the following characters: a-zA-Z0-9-_"
	storageInitSvcPrompt     = "Which " + color.Emphasize("service") + " would you like to associate with this storage resource?"
	storageInitSvcHelp       = `The service you'd like to have access to this storage resource. 
We'll deploy the Cloudformation for the storage when you run 'svc deploy'.`
)

var fmtStorageInitCreateConfirm = "Okay, we'll create %s %s named %s linked to your %s service."

type s3Config struct {
	// TODO add website config
}

type initStorageVars struct {
	*GlobalOpts
	StorageType string
	StorageName string
	StorageSvc  string
}

type initStorageOpts struct {
	initStorageVars

	fs          afero.Fs
	ws          wsSvcReader
	addonWriter wsAddonWriter
	store       store

	app *config.Application
}

func newStorageInitOpts(vars initStorageVars) (*initStorageOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store client: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	return &initStorageOpts{
		initStorageVars: vars,

		fs:            &afero.Afero{Fs: afero.NewOsFs()},
		store:         store,
		ws:            ws,
		wsAddonWriter: ws,
	}, nil
}

func (o *initStorageOpts) Validate() error {
	if o.AppName() == "" {
		return errNoAppInWorkspace
	}
	if o.StorageSvc != "" {
		if err := o.validateServiceName(); err != nil {
			return err
		}
	}

	if o.StorageType != "" {
		if err := validateStorageType(o.StorageType); err != nil {
			return err
		}
	}

	if o.StorageName != "" {
		var err error
		switch o.StorageType {
		case dynamoDBStorageType:
			err = dynamoTableNameValidation(o.StorageName)
		case s3StorageType:
			err = s3BucketNameValidation(o.StorageName)
		default:
			// use dynamo since it's a superset of s3
			err = dynamoTableNameValidation(o.StorageName)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *initStorageOpts) Ask() error {
	if err := o.askStorageSvc(); err != nil {
		return err
	}
	if err := o.askStorageType(); err != nil {
		return err
	}
	if err := o.askStorageName(); err != nil {
		return err
	}

	return nil
}

func (o *initStorageOpts) askStorageType() error {
	if o.StorageType != "" {
		return nil
	}

	storageType, err := o.prompt.SelectOne(fmt.Sprintf(
		fmtStorageInitTypePrompt, color.HighlightUserInput(o.StorageSvc)),
		storageInitTypeHelp,
		storageTypes)
	if err != nil {
		return fmt.Errorf("select storage type: %w", err)
	}

	o.StorageType = storageType
	return nil
}

func (o *initStorageOpts) askStorageName() error {
	if o.StorageName != "" {
		return nil
	}
	var validator func(interface{}) error
	switch o.StorageType {
	case dynamoDBStorageType:
		validator = dynamoTableNameValidation
	case s3StorageType:
		validator = s3BucketNameValidation
	}
	name, err := o.prompt.Get(fmt.Sprintf(fmtStorageInitNamePrompt,
		color.HighlightUserInput(o.StorageType)),
		storageInitNameHelp,
		validator)

	if err != nil {
		return fmt.Errorf("input storage name: %w", err)
	}
	o.StorageName = name
	return nil
}

func (o *initStorageOpts) askStorageSvc() error {
	if o.StorageSvc != "" {
		return nil
	}
	localSvcNames, err := o.ws.ServiceNames()
	if err != nil {
		return fmt.Errorf("retrieve local service names: %w", err)
	}
	svc, err := o.prompt.SelectOne(storageInitSvcPrompt,
		storageInitSvcHelp,
		localSvcNames,
	)
	o.StorageSvc = svc
	return nil
}

func (o *initStorageOpts) validateServiceName() error {
	names, err := o.ws.ServiceNames()
	if err != nil {
		return fmt.Errorf("retrieve local service names: %w", err)
	}
	for _, name := range names {
		if o.StorageSvc == name {
			return nil
		}
	}
	return fmt.Errorf("service %s not found in the workspace", o.StorageSvc)
}

func (o *initStorageOpts) Execute() error {
	params := `
App:
  Type: String
  Description: Your application's name.
Env:
  Type: String
  Description: The environment name your service, job, or workflow is being deployed to.
Name:
  Type: String
  Description: The name of the service, job, or workflow being deployed.`
	output := `
%[1]sBucketName:
  Description: "The name of a user-defined bucket."
  Value: !Ref %[1]s
%[1]sAccessPolicy:
  Description: "The IAM::ManagedPolicy to attach to the task role"
  Value: !Ref %[1]sAccessPolicy`
	policy := `
%[1]sAccessPolicy:
  Type: AWS::IAM::ManagedPolicy
  Properties:
    Description: !Sub
      - Grants CRUD access to the S3 bucket ${Bucket}
      - { Bucket: !Ref %[1]s }
    PolicyDocument:
      Version: 2012-10-17
      Statement:
        - Sid: S3ObjectActions
          Effect: Allow
          Action:
          - s3:GetObject
          - s3:PutObject
          - s3:PutObjectACL
          - s3:PutObjectTagging
          - s3:DeleteObject
          - s3:RestoreObject
          Resource: !Sub ${%[1]s.Arn}/*
        - Sid: S3ListAction
          Effect: Allow
          Action: s3:ListBucket
          Resource: !Sub ${%[1]s.Arn}`
	cf := `
%[2]s:
  Type: AWS::S3::Bucket
  DeletionPolicy: Retain
  Properties:
    AccessControl: Private
    BucketEncryption: 
      ServerSideEncryptionConfiguration:
      - ServerSideEncryptionByDefault:
          SSEAlgorithm: AES256
    BucketName: !Sub '${App}-${Env}-${Name}-%[1]s'
    PublicAccessBlockConfiguration: 
      BlockPublicAcls: true
	  BlockPublicPolicy: true`
	logicalIdName := logicalIdSafe(o.StorageName)
	output = fmt.Sprintf(output, logicalIdName)
	policy = fmt.Sprintf(policy, logicalIdName)
	cf = fmt.Sprintf(cf, o.StorageName, logicalIdName)

	paramsOut := &template.Content{
		Buffer: bytes.NewBufferString(params),
	}
	outputOut := &template.Content{
		Buffer: bytes.NewBufferString(output),
	}
	policyOut := &template.Content{
		Buffer: bytes.NewBufferString(policy),
	}
	cfOut := &template.Content{
		Buffer: bytes.NewBufferString(cf),
	}

	wsAddonWriter.WriteAddons(paramsOut, o.StorageSvc, "params.yaml")
	wsAddonWriter.WriteAddons(outputOut, o.StorageSvc, "outputs.yaml")
	wsAddonWriter.WriteAddons(policyOut, o.StorageSvc, "policy.yaml")
	wsAddonWriter.WriteAddons(cfOut, o.StorageSvc, "s3.yaml")

}

// Strip non-alphanumeric characters from an input string
func logicalIdSafe(input string) string {
	var output string
	for _, c := range input {
		if c >= 'a' && c <= 'z' {
			output += string(c)
		} else if c >= '0' && c <= '9' {
			output += string(c)
		} else if c >= 'A' && c <= 'Z' {
			output += string(c)
		} else {
			continue
		}
	}
	return output
}

func BuildStorageInitCmd() *cobra.Command {
	vars := initStorageVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Hidden: true, //TODO remove when ready for production
		Use:    "init",
		Short:  "Creates a new storage table in an environment.",
		Example: `
  Create a "my-table" DynamoDB table in the "test" environment.
  /code $ copilot storage init --name my-table --storage-type dynamo-db --svc frontend`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newStorageInitOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}
			log.Infoln("Recommended follow-up actions:")
			// for _, followup := range opts.RecommendedActions() {
			// 	log.Infof("- %s\n", followup)
			// }
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.StorageName, nameFlag, nameFlagShort, "", storageFlagDescription)
	cmd.Flags().StringVar(&vars.StorageType, storageTypeFlag, "", storageTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.StorageSvc, svcFlag, svcFlagShort, "", storageServiceFlagDescription)

	return cmd
}
