// Copyright Amazon.com, Inc or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"regexp"

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

const (
	dynamoDBTableFriendlyText = "DynamoDB Table"
	s3BucketFriendlyText      = "S3 Bucket"
	lsiFriendlyText           = "Local Secondary Index"
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

var fmtStorageInitDynamoInfoText = `We're going to ask you some questions about your database schema and datatypes.
You can change any of this information at the end by editing the CloudFormation
generated at ./ecs-project/%s/addons/%s-%s.yml.`

var (
	fmtStorageInitDDBKeyPrompt  = "What would you like to name the %s of this %s?"
	storageInitDDBKeyTypePrompt = "What datatype is this key?"
	storageInitDDBLSIPrompt     = "Would you like to add a local secondary index to this table?"
	storageInitDDBLSINamePrompt = "What would you like to name this " + color.Emphasize("LSI") + "?"
)

var (
	storageInitDDBPartitionKeyPromptHelp = "The partition key of this table. This key, along with the sort key, will make up the primary key."
	storageInitDDBSortKeyPromptHelp      = "The sort key of this table. Without a sort key, the partition key " + color.Emphasize("must") + " be unique on the table."
	storageInitDDBLSISortKeyPromptHelp   = "The sort key of this Local Secondary Index. An LSI can be queried based on the partition key and LSI sort key."
	storageInitDDBKeyTypeHelp            = "The datatype to store in the key. N is number, S is string, B is binary."
	storageInitDDBLSIHelp                = "A Local Secondary Index has the same partition key as the table, but a different sort key."
)

const (
	StringType = "S"
	IntType    = "N"
	BinaryType = "B"
)

var attributeTypes = []string{
	StringType,
	IntType,
	BinaryType,
}

var attributeTypesLong = []string{
	"String",
	"Number",
	"Binary",
}

const (
	PartitionKeyType = "HASH"
	SortKeyType      = "RANGE"
)

type dynamoDBConfig struct {
	PartitionKey dynamoKey
	SortKey      dynamoKey
	LSIs         []localSecondaryIndex
	Attributes   []attribute
}

type dynamoKey struct {
	Name string
	Type string
}

type attribute struct {
	Name     string
	DataType string
}

type localSecondaryIndex struct {
	Name         string
	PartitionKey dynamoKey
	SortKey      dynamoKey
	Attributes   []attribute
}

type s3Config struct {
	// TODO add website config
}

type initStorageVars struct {
	*GlobalOpts
	StorageType  string
	StorageName  string
	StorageSvc   string
	PartitionKey string
	SortKey      string
	Attributes   []string
	LSISort      string
	LSIName      string
	GSIPartition string
	GSISort      string
}

type initStorageOpts struct {
	initStorageVars

	fs    afero.Fs
	ws    wsAddonManager
	store store

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

		fs:    &afero.Afero{Fs: afero.NewOsFs()},
		store: store,
		ws:    ws,
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

	// if o.PartitionKey != "" {
	// 	// TODO
	// 	if err := validateKey(o.partitionKey); err != nil {

	// 	}
	// }

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
	switch o.StorageType {
	case dynamoDBStorageType:
		if err := o.askDynamoPartitionKey(); err != nil {
			return err
		}
		if err := o.askDynamoSortKey(); err != nil {
			return err
		}
		if err := o.askDynamoLSIConfig(); err != nil {
			return err
		}
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
	var friendlyText string
	switch o.StorageType {
	case s3StorageType:
		validator = s3BucketNameValidation
		friendlyText = s3BucketFriendlyText
	case dynamoDBStorageType:
		validator = dynamoTableNameValidation
		friendlyText = dynamoDBTableFriendlyText
	}

	name, err := o.prompt.Get(fmt.Sprintf(fmtStorageInitNamePrompt,
		color.HighlightUserInput(friendlyText)),
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

func (o *initStorageOpts) askDynamoPartitionKey() error {
	if o.PartitionKey != "" {
		return nil
	}
	keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
		color.HighlightUserInput("partition key"),
		color.HighlightUserInput(dynamoDBStorageType),
	)
	key, err := o.prompt.Get(keyPrompt,
		storageInitDDBPartitionKeyPromptHelp,
		basicNameValidation,
	)
	if err != nil {
		return fmt.Errorf("get ddb partition key: %w", err)
	}
	keyType, err := o.prompt.SelectOne(storageInitDDBKeyTypePrompt,
		storageInitDDBKeyTypeHelp,
		attributeTypesLong,
	)
	o.PartitionKey = key + ":" + keyType
	return nil
}

func (o *initStorageOpts) askDynamoSortKey() error {
	if o.SortKey != "" {
		return nil
	}
	keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
		color.HighlightUserInput("sort key"),
		color.HighlightUserInput(dynamoDBStorageType),
	)
	key, err := o.prompt.Get(keyPrompt,
		storageInitDDBSortKeyPromptHelp,
		basicNameValidation,
	)
	if err != nil {
		return fmt.Errorf("get ddb sort key: %w", err)
	}
	keyType, err := o.prompt.SelectOne(storageInitDDBKeyTypePrompt,
		storageInitDDBKeyTypeHelp,
		attributeTypesLong,
	)
	o.SortKey = key + ":" + keyType
	return nil
}

func (o *initStorageOpts) askDynamoLSIConfig() error {
	if o.LSIName != "" || o.LSISort != "" {
		return nil
	}
	addLsi, err := o.prompt.Confirm(
		storageInitDDBLSIPrompt,
		storageInitDDBLSIHelp,
	)
	if err != nil {
		return fmt.Errorf("confirm add lsi to table: %w", err)
	}
	if addLsi != true {
		return nil
	}

	name, err := o.prompt.Get(storageInitDDBLSINamePrompt,
		storageInitDDBLSIHelp,
		dynamoTableNameValidation,
	)
	if err != nil {
		return fmt.Errorf("get lsi name: %w", err)
	}
	o.LSIName = name

	keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
		color.HighlightUserInput("sort key"),
		color.HighlightUserInput(lsiFriendlyText),
	)
	key, err := o.prompt.Get(keyPrompt,
		storageInitDDBLSISortKeyPromptHelp,
		dynamoTableNameValidation,
	)
	if err != nil {
		return fmt.Errorf("get lsi sort key: %w", err)
	}

	keyType, err := o.prompt.SelectOne(storageInitDDBKeyTypePrompt,
		storageInitDDBLSISortKeyPromptHelp,
		attributeTypesLong,
	)
	o.LSISort = key + ":" + keyType
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
	cf := `%[2]s:
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
	logicalIDName := logicalIDSafe(o.StorageName)
	output = fmt.Sprintf(output, logicalIDName)
	policy = fmt.Sprintf(policy, logicalIDName)
	cf = fmt.Sprintf(cf, o.StorageName, logicalIDName)

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

	o.ws.WriteAddon(paramsOut, o.StorageSvc, "params.yaml")
	o.ws.WriteAddon(outputOut, o.StorageSvc, "outputs.yaml")
	o.ws.WriteAddon(policyOut, o.StorageSvc, "policy.yaml")
	o.ws.WriteAddon(cfOut, o.StorageSvc, "s3.yaml")
	return nil
}

var nonAlphaNum = regexp.MustCompile("[^a-zA-Z0-9]+")

// Strip non-alphanumeric characters from an input string
func logicalIDSafe(input string) string {
	return nonAlphaNum.ReplaceAllString(input, "")
}

func (o *initStorageOpts) generateDynamoDBConfig() (*dynamoDBConfig, error) {
	// TODO fill in as needed
	return nil, nil
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
