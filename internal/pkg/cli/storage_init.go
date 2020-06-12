// Copyright Amazon.com, Inc or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/selector"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	dynamoDBStorageType = "DynamoDB"
	s3StorageType       = "S3"
)

const (
	s3BucketFriendlyText      = "S3 Bucket"
	dynamoDBTableFriendlyText = "DynamoDB Table"
	lsiFriendlyText           = "Local Secondary Index"
)

const (
	ddbKeyString       = "key"
	ddbAttributeString = "attribute"
)

var storageTypes = []string{
	dynamoDBStorageType,
	s3StorageType,
}

// General-purpose prompts, collected for all storage resources.
var (
	fmtStorageInitTypePrompt = "What " + color.Emphasize("type") + " of storage would you like to associate with %s?"
	storageInitTypeHelp      = `The type of storage you'd like to add to your service. 
DynamoDB is a key-value and document database that delivers single-digit millisecond performance at any scale.
S3 is a web object store built to store and retrieve any amount of data from anywhere on the Internet.`

	fmtStorageInitNamePrompt = "What would you like to " + color.Emphasize("name") + " this %s?"
	storageInitNameHelp      = "The name of this storage resource. You can use the following characters: a-zA-Z0-9-_"

	storageInitSvcPrompt = "Which " + color.Emphasize("service") + " would you like to associate with this storage resource?"
	storageInitSvcHelp   = `The service you'd like to have access to this storage resource. 
We'll deploy the Cloudformation for the storage when you run 'svc deploy'.`
)

var fmtStorageInitCreateConfirm = "Okay, we'll create %s %s named %s linked to your %s service."

// DDB-specific questions and help prompts.
var (
	fmtStorageInitDDBKeyPrompt     = "What would you like to name the %s of this %s?"
	storageInitDDBPartitionKeyHelp = "The partition key of this table. This key, along with the sort key, will make up the primary key."

	storageInitDDBSortKeyConfirm = "Would you like to add a sort key to this table?"
	storageInitDDBSortKeyHelp    = "The sort key of this table. Without a sort key, the partition key " + color.Emphasize("must") + ` be unique on the table.
You must specify a sort key if you wish to add alternate sort keys later.`

	fmtStorageInitDDBKeyTypePrompt = "What datatype is this %s?"
	fmtStorageInitDDBKeyTypeHelp   = "The datatype to store in the %s. N is number, S is string, B is binary."

	storageInitDDBMoreAttributesPrompt = "Would you like to add more attributes to this table?"
	storageInitDDBMoreAttributesHelp   = "You can use these extra attributes to specify additional sort keys or alternate partition keys."
	storageInitDDBAttributePrompt      = "What would you like to name this " + color.Emphasize(ddbAttributeString) + "?"
	storageInitDDBAttributeHelp        = "You can use the the characters [a-zA-Z0-9.-_]"

	storageInitDDBLSIPrompt = "Would you like to add any alternate sort keys to this table?"
	storageInitDDBLSIHelp   = `Alternate sort keys create Local Secondary Indexes, which allow you to sort the table using the same 
partition key but a different sort key.`

	storageInitDDBLSINamePrompt  = "What would you like to name this " + color.Emphasize("LSI") + "?"
	storageInitDDBLSISortKeyHelp = "The sort key of this Local Secondary Index. An LSI can be queried based on the partition key and LSI sort key."
)

const (
	ddbStringType = "S"
	ddbIntType    = "N"
	ddbBinaryType = "B"
)

var attributeTypes = []string{
	ddbStringType,
	ddbIntType,
	ddbBinaryType,
}

var attributeTypesLong = []string{
	"String",
	"Number",
	"Binary",
}

const (
	ddbPartitionKeyType = "HASH"
	ddbSortKeyType      = "RANGE"
)

type dynamoDBConfig struct {
	tableName    string
	partitionKey string
	sortKey      string
	hasLsi       bool
	lsis         []localSecondaryIndex
	attributes   []attribute
}

type attribute struct {
	name        string
	ddbDataType string
}

type localSecondaryIndex struct {
	name         string
	partitionKey string
	sortKey      string
	attributes   []attribute
}

type s3Config struct {
	// TODO add website config
}

type initStorageVars struct {
	*GlobalOpts
	storageType string
	storageName string
	storageSvc  string

	// Dynamo DB specific values collected via flags or prompts
	partitionKey string
	sortKey      string
	attributes   []string // Attributes collected as "attName:T" where T is one of [SNB]
	lsiSorts     []string // lsi sort keys collected as names of existing attributes
	noLsi        bool
	noSort       bool
}

type initStorageOpts struct {
	initStorageVars

	fs    afero.Fs
	ws    wsAddonManager
	store store

	app *config.Application
	sel configSelector
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
		sel:   selector.NewConfigSelect(vars.prompt, store),
	}, nil
}

func (o *initStorageOpts) Validate() error {
	if o.AppName() == "" {
		return errNoAppInWorkspace
	}
	if o.storageSvc != "" {
		if err := o.validateServiceName(); err != nil {
			return err
		}
	}
	if o.storageType != "" {
		if err := validateStorageType(o.storageType); err != nil {
			return err
		}
	}
	if o.storageName != "" {
		var err error
		switch o.storageType {
		case dynamoDBStorageType:
			err = dynamoTableNameValidation(o.storageName)
		case s3StorageType:
			err = s3BucketNameValidation(o.storageName)
		default:
			// use dynamo since it's a superset of s3
			err = dynamoTableNameValidation(o.storageName)
		}
		if err != nil {
			return err
		}
	}
	if o.partitionKey != "" {
		if err := validateKey(o.partitionKey); err != nil {
			return err
		}
	}
	if o.sortKey != "" {
		if err := validateKey(o.sortKey); err != nil {
			return err
		}
	}
	if len(o.attributes) != 0 {
		if err := validateAttributes(o.attributes); err != nil {
			return err
		}
	}
	// --no-lsi and --lsi are mutually exclusive.
	if o.noLsi && len(o.lsiSorts) != 0 {
		return fmt.Errorf("validate LSI configuration: cannot specify --no-lsi and --lsi options at once")
	}

	// --no-sort and --lsi are mutually exclusive.
	if o.noSort && len(o.lsiSorts) != 0 {
		return fmt.Errorf("validate LSI configuration: cannot specify --no-sort and --lsi options at once")
	}
	if len(o.lsiSorts) != 0 {
		if err := validateLsi(o.lsiSorts, o.attributes); err != nil {
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
	switch o.storageType {
	case dynamoDBStorageType:
		if err := o.askDynamoPartitionKey(); err != nil {
			return err
		}
		if err := o.askDynamoSortKey(); err != nil {
			return err
		}
		if err := o.askDynamoAttributes(); err != nil {
			return err
		}
		if err := o.askDynamoLSIConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (o *initStorageOpts) askStorageType() error {
	if o.storageType != "" {
		return nil
	}

	storageType, err := o.prompt.SelectOne(fmt.Sprintf(
		fmtStorageInitTypePrompt, color.HighlightUserInput(o.storageSvc)),
		storageInitTypeHelp,
		storageTypes)
	if err != nil {
		return fmt.Errorf("select storage type: %w", err)
	}

	o.storageType = storageType
	return nil
}

func (o *initStorageOpts) askStorageName() error {
	if o.storageName != "" {
		return nil
	}
	var validator func(interface{}) error
	var friendlyText string
	switch o.storageType {
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
	o.storageName = name
	return nil
}

func (o *initStorageOpts) askStorageSvc() error {
	if o.storageSvc != "" {
		return nil
	}
	svc, err := o.sel.Service(storageInitSvcPrompt,
		storageInitSvcHelp,
		o.AppName(),
	)
	if err != nil {
		return fmt.Errorf("retrieve local service names: %w", err)
	}
	o.storageSvc = svc
	return nil
}

func (o *initStorageOpts) askDynamoPartitionKey() error {
	if o.partitionKey != "" {
		return nil
	}
	keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
		color.HighlightUserInput("partition key"),
		color.HighlightUserInput(dynamoDBStorageType),
	)
	key, err := o.prompt.Get(keyPrompt,
		storageInitDDBPartitionKeyHelp,
		dynamoTableNameValidation,
	)
	if err != nil {
		return fmt.Errorf("get DDB partition key: %w", err)
	}

	keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
	keyTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, ddbKeyString)

	keyType, err := o.prompt.SelectOne(keyTypePrompt,
		keyTypeHelp,
		attributeTypesLong,
	)
	if err != nil {
		return fmt.Errorf("get DDB partition key datatype: %w", err)
	}

	o.partitionKey = key + ":" + keyType
	return nil
}

func (o *initStorageOpts) askDynamoSortKey() error {
	if o.sortKey != "" {
		return nil
	}
	// If the user has not specified a sort key and has specified the --no-sort flag we don't have to demand it of them.
	if o.noSort {
		return nil
	}

	response, err := o.prompt.Confirm(storageInitDDBSortKeyConfirm, storageInitDDBSortKeyHelp)
	if err != nil {
		return fmt.Errorf("confirm DDB sort key: %w", err)
	}
	if response == false {
		o.noSort = true
		return nil
	}

	keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
		color.HighlightUserInput("sort key"),
		color.HighlightUserInput(dynamoDBStorageType),
	)
	key, err := o.prompt.Get(keyPrompt,
		storageInitDDBSortKeyHelp,
		dynamoTableNameValidation,
	)
	if err != nil {
		return fmt.Errorf("get DDB sort key: %w", err)
	}
	keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
	keyTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, ddbKeyString)

	keyType, err := o.prompt.SelectOne(keyTypePrompt,
		keyTypeHelp,
		attributeTypesLong,
	)
	if err != nil {
		return fmt.Errorf("get DDB sort key datatype: %w", err)
	}
	o.sortKey = key + ":" + keyType
	return nil
}
func (o *initStorageOpts) askDynamoAttributes() error {
	// Extra attributes have been specified by flags.
	if len(o.attributes) != 0 {
		return nil
	}
	// If --no-lsi has been specified, there is no need to ask for more attributes
	if o.noLsi {
		return nil
	}

	// If --no-sort has been specified, there is no need to ask for more attributes
	if o.noSort {
		return nil
	}
	attributeTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, color.Emphasize(ddbAttributeString))
	attributeTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, ddbAttributeString)
	for {
		moreAtt, err := o.prompt.Confirm(storageInitDDBMoreAttributesPrompt, storageInitDDBMoreAttributesHelp)
		if err != nil {
			return fmt.Errorf("confirm add more attributes: %w", err)
		}
		if !moreAtt {
			break
		}
		att, err := o.prompt.Get(storageInitDDBAttributePrompt,
			storageInitDDBAttributeHelp,
			dynamoTableNameValidation)
		if err != nil {
			return fmt.Errorf("get DDB attribute name: %w", err)
		}
		attType, err := o.prompt.SelectOne(attributeTypePrompt,
			attributeTypeHelp,
			attributeTypesLong,
		)
		if err != nil {
			return fmt.Errorf("get DDB attribute type: %w", err)
		}

		o.attributes = append(o.attributes, att+":"+attType)
	}
	return nil
}

func (o *initStorageOpts) askDynamoLSIConfig() error {
	if o.noLsi {
		return nil
	}
	if o.noSort {
		return nil
	}
	addLsi, err := o.prompt.Confirm(
		storageInitDDBLSIPrompt,
		storageInitDDBLSIHelp,
	)
	if err != nil {
		return fmt.Errorf("confirm add LSI to table: %w", err)
	}
	o.noLsi = !addLsi
	if addLsi != true {
		return nil
	}

	var attributeNames []string
	for _, att := range o.attributes {
		attributeParts, err := getAttrFromKey(att)
		if err != nil {
			return fmt.Errorf("get name from attribute: %w", err)
		}
		attributeNames = append(attributeNames, attributeParts.name)
	}

	names, err := o.prompt.MultiSelect(storageInitDDBLSINamePrompt,
		storageInitDDBLSIHelp,
		attributeNames,
	)
	if err != nil {
		return fmt.Errorf("get LSI sort keys: %w", err)
	}
	if len(names) > 5 {
		return fmt.Errorf("cannot specify more than 5 alternate sort keys")
	}
	o.lsiSorts = names
	return nil
}

func (o *initStorageOpts) validateServiceName() error {
	names, err := o.ws.ServiceNames()
	if err != nil {
		return fmt.Errorf("retrieve local service names: %w", err)
	}
	for _, name := range names {
		if o.storageSvc == name {
			return nil
		}
	}
	return fmt.Errorf("service %s not found in the workspace", o.storageSvc)
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
	logicalIDName := logicalIDSafe(o.storageName)
	output = fmt.Sprintf(output, logicalIDName)
	policy = fmt.Sprintf(policy, logicalIDName)
	cf = fmt.Sprintf(cf, o.storageName, logicalIDName)

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

	o.ws.WriteAddon(paramsOut, o.storageSvc, "params.yaml")
	o.ws.WriteAddon(outputOut, o.storageSvc, "outputs.yaml")
	o.ws.WriteAddon(policyOut, o.storageSvc, "policy.yaml")
	o.ws.WriteAddon(cfOut, o.storageSvc, "s3.yaml")
	return nil
}

var nonAlphaNum = regexp.MustCompile("[^a-zA-Z0-9]+")

// Strip non-alphanumeric characters from an input string
func logicalIDSafe(input string) string {
	return nonAlphaNum.ReplaceAllString(input, "")
}

func (o *initStorageOpts) generateDynamoDBConfig() (*dynamoDBConfig, error) {
	cfg := &dynamoDBConfig{}
	cfg.tableName = o.storageName
	hashAttr, err := getAttrFromKey(o.partitionKey)
	if err != nil {
		return nil, err
	}
	rangeAttr, err := getAttrFromKey(o.sortKey)
	if err != nil {
		return nil, err
	}
	cfg.attributes = []attribute{
		hashAttr,
		rangeAttr,
	}
	if len(o.lsiSorts) != 0 {
		for _, k := range o.lsiSorts {
			lsiAttr, err := getAttrFromKey(k)
			if err != nil {
				return nil, err
			}
			cfg.attributes = append(cfg.attributes, lsiAttr)
		}
	}
	cfg.partitionKey = hashAttr.name
	cfg.sortKey = rangeAttr.name

	return nil, nil
}

var regexpMatchAttribute = regexp.MustCompile("^(\\S+):([sbnSBN])")

func getAttrFromKey(input string) (attribute, error) {
	attrs := regexpMatchAttribute.FindStringSubmatch(input)
	if len(attrs) == 0 {
		return attribute{}, fmt.Errorf("parse attribute from key: %s", input)
	}
	return attribute{
		name:        attrs[1],
		ddbDataType: strings.ToUpper(attrs[2]),
	}, nil
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
	cmd.Flags().StringVarP(&vars.storageName, nameFlag, nameFlagShort, "", storageFlagDescription)
	cmd.Flags().StringVarP(&vars.storageType, storageTypeFlag, svcTypeFlagShort, "", storageTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.storageSvc, svcFlag, svcFlagShort, "", storageServiceFlagDescription)

	cmd.Flags().StringVar(&vars.partitionKey, storagePartitionKeyFlag, "", storagePartitionKeyFlagDescription)
	cmd.Flags().StringVar(&vars.sortKey, storageSortKeyFlag, "", storageSortKeyFlagDescription)
	cmd.Flags().StringArrayVar(&vars.attributes, storageAttributeFlag, []string{}, storageAttributeFlagDescription)
	cmd.Flags().StringArrayVar(&vars.lsiSorts, storageLSIConfigFlag, []string{}, storageLSIConfigFlagDescription)
	cmd.Flags().BoolVar(&vars.noLsi, storageNoLSIFlag, false, storageNoLsiFlagDescription)
	cmd.Flags().BoolVar(&vars.noSort, storageNoSortFlag, false, storageNoSortFlagDescription)

	requiredFlags := pflag.NewFlagSet("Required Flags", pflag.ContinueOnError)
	requiredFlags.AddFlag(cmd.Flags().Lookup(nameFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(storageTypeFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(svcFlag))

	ddbFlags := pflag.NewFlagSet("DynamoDB Flags", pflag.ContinueOnError)
	ddbFlags.AddFlag(cmd.Flags().Lookup(storagePartitionKeyFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageSortKeyFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageNoSortFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageAttributeFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageLSIConfigFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageNoLSIFlag))
	cmd.Annotations = map[string]string{
		// The order of the sections we want to display.
		"sections":       `Required,DynamoDB Flags`,
		"Required":       requiredFlags.FlagUsages(),
		"DynamoDB Flags": ddbFlags.FlagUsages(),
	}
	cmd.SetUsageTemplate(`{{h1 "Usage"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{$annotations := .Annotations}}{{$sections := split .Annotations.sections ","}}{{if gt (len $sections) 0}}

{{range $i, $sectionName := $sections}}{{h1 (print $sectionName " Flags")}}
{{(index $annotations $sectionName) | trimTrailingWhitespaces}}{{if ne (inc $i) (len $sections)}}

{{end}}{{end}}{{end}}{{if .HasAvailableInheritedFlags}}

{{h1 "Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{h1 "Examples"}}{{code .Example}}{{end}}
`)
	return cmd
}
