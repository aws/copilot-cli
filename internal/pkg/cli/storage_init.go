// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding"
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/dustin/go-humanize/english"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	dynamoDBStorageType = "DynamoDB"
	s3StorageType       = "S3"
	rdsStorageType      = "Aurora"
)

var storageTypes = []string{
	dynamoDBStorageType,
	s3StorageType,
	rdsStorageType,
}

// Displayed options for storage types
const (
	dynamoDBStorageTypeOption = "DynamoDB"
	s3StorageTypeOption       = "S3"
	rdsStorageTypeOption      = "Aurora Serverless"
)

const (
	s3BucketFriendlyText      = "S3 Bucket"
	dynamoDBTableFriendlyText = "DynamoDB Table"
	rdsFriendlyText           = "Database Cluster"
)

const (
	lifecycleEnvironmentLevel = "environment"
	lifecycleWorkloadLevel    = "workload"

	lifecycleEnvironmentFriendlyText = "No, the storage should be created and deleted at the environment level"
	fmtLifecycleWorkloadFriendlyText = "Yes, the storage should be created and deleted at the same time as %s"
)

var validLifecycleOptions = []string{lifecycleWorkloadLevel, lifecycleEnvironmentLevel}

// General-purpose prompts, collected for all storage resources.
var (
	storageInitTypeHelp = `The type of storage you'd like to add to your workload.
DynamoDB is a key-value and document database that delivers single-digit millisecond performance at any scale.
S3 is a web object store built to store and retrieve any amount of data from anywhere on the Internet.
Aurora Serverless is an on-demand autoscaling configuration for Amazon Aurora, a MySQL and PostgreSQL-compatible relational database.
`

	fmtStorageInitNamePrompt = "What would you like to " + color.Emphasize("name") + " this %s?"
	storageInitNameHelp      = "The name of this storage resource. You can use the following characters: a-zA-Z0-9-_"

	storageInitSvcPrompt = "Which " + color.Emphasize("workload") + " needs access to the storage?"

	fmtStorageInitLifecyclePrompt = "Do you want the storage to be created and deleted with the %s service?"
)

// DDB-specific questions and help prompts.
var (
	fmtStorageInitDDBKeyPrompt     = "What would you like to name the %s of this %s?"
	storageInitDDBPartitionKeyHelp = "The partition key of this table. This key, along with the sort key, will make up the primary key."

	storageInitDDBSortKeyConfirm = "Would you like to add a sort key to this table?"
	storageInitDDBSortKeyHelp    = "The sort key of this table. Without a sort key, the partition key " + color.Emphasize("must") + ` be unique on the table.
You must specify a sort key if you wish to add alternate sort keys later.`

	fmtStorageInitDDBKeyTypePrompt = "What datatype is this %s?"
	fmtStorageInitDDBKeyTypeHelp   = "The datatype to store in the %s. N is number, S is string, B is binary."

	storageInitDDBLSIPrompt = "Would you like to add any alternate sort keys to this table?"
	storageInitDDBLSIHelp   = `Alternate sort keys create Local Secondary Indexes, which allow you to sort the table using the same 
partition key but a different sort key. You may specify up to 5 alternate sort keys.`

	storageInitDDBMoreLSIPrompt = "Would you like to add more alternate sort keys to this table?"

	storageInitDDBLSINamePrompt = "What would you like to name this " + color.Emphasize("alternate sort key") + "?"
	storageInitDDBLSINameHelp   = "You can use the characters [a-zA-Z0-9.-_]"
)

// DynamoDB specific constants and variables.
const (
	ddbKeyString  = "key"
	ddbStringType = "String"
	ddbIntType    = "Number"
	ddbBinaryType = "Binary"
)

var attributeTypes = []string{
	ddbStringType,
	ddbIntType,
	ddbBinaryType,
}

// RDS Aurora Serverless specific questions and help prompts.
var (
	storageInitRDSInitialDBNamePrompt = "What would you like to name the initial database in your cluster?"
	storageInitRDSDBEnginePrompt      = "Which database engine would you like to use?"
)

// RDS Aurora Serverless specific constants and variables.
const (
	auroraServerlessVersionV1      = "v1"
	auroraServerlessVersionV2      = "v2"
	defaultAuroraServerlessVersion = auroraServerlessVersionV2

	fmtRDSStorageNameDefault = "%s-cluster"

	engineTypeMySQL      = addon.RDSEngineTypeMySQL
	engineTypePostgreSQL = addon.RDSEngineTypePostgreSQL
)

var auroraServerlessVersions = []string{
	auroraServerlessVersionV1,
	auroraServerlessVersionV2,
}

var engineTypes = []string{
	engineTypeMySQL,
	engineTypePostgreSQL,
}

const workloadTypeNonLocal = "Non Local"

const (
	blobDescriptionParameters = "parameters"
	blobDescriptionTemplate   = "template"
)

type initStorageVars struct {
	storageType    string
	storageName    string
	workloadName   string
	lifecycle      string
	addIngressFrom string

	// Dynamo DB specific values collected via flags or prompts
	partitionKey string
	sortKey      string
	lsiSorts     []string // lsi sort keys collected as "name:T" where T is one of [SNB]
	noLSI        bool
	noSort       bool

	// RDS Aurora Serverless specific values collected via flags or prompts
	auroraServerlessVersion string
	rdsEngine               string
	rdsParameterGroup       string
	rdsInitialDBName        string
}

type initStorageOpts struct {
	initStorageVars
	appName string

	fs    afero.Fs
	ws    wsReadWriter
	store store

	sel       wsSelector
	configSel configSelector
	prompt    prompter

	// Cached data.
	workloadType   string
	workloadExists bool
}

func newStorageInitOpts(vars initStorageVars) (*initStorageOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("storage init"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	fs := afero.NewOsFs()
	ws, err := workspace.Use(fs)
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	return &initStorageOpts{
		initStorageVars: vars,
		appName:         tryReadingAppName(),

		fs:        fs,
		store:     store,
		ws:        ws,
		sel:       selector.NewLocalWorkloadSelector(prompter, store, ws, selector.OnlyInitializedWorkloads),
		configSel: selector.NewConfigSelector(prompter, store),
		prompt:    prompter,
	}, nil
}

// Validate returns an error for any invalid optional flags.
func (o *initStorageOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.addIngressFrom != "" {
		if err := o.validateAddIngressFrom(); err != nil {
			return err
		}
	}
	// --no-lsi and --lsi are mutually exclusive.
	if o.noLSI && len(o.lsiSorts) != 0 {
		return fmt.Errorf("validate LSI configuration: cannot specify --no-lsi and --lsi options at once")
	}
	// --no-sort and --lsi are mutually exclusive.
	if o.noSort && len(o.lsiSorts) != 0 {
		return fmt.Errorf("validate LSI configuration: cannot specify --no-sort and --lsi options at once")
	}
	if o.auroraServerlessVersion != "" {
		if err := o.validateServerlessVersion(); err != nil {
			return err
		}
	}
	return nil
}

func (o *initStorageOpts) validateAddIngressFrom() error {
	if o.workloadName != "" {
		return fmt.Errorf("--%s cannot be specified with --%s", workloadFlag, storageAddIngressFromFlag)
	}
	if o.lifecycle == lifecycleWorkloadLevel {
		return fmt.Errorf("--%s cannot be %s when --%s is used", storageLifecycleFlag, lifecycleWorkloadLevel, storageAddIngressFromFlag)
	}
	if o.storageName == "" {
		return fmt.Errorf("--%s is required when --%s is used", nameFlag, storageAddIngressFromFlag)
	}
	if o.storageType == "" {
		return fmt.Errorf("--%s is required when --%s is used", storageTypeFlag, storageAddIngressFromFlag)
	}
	exist, err := o.ws.WorkloadExists(o.addIngressFrom)
	if err != nil {
		return fmt.Errorf("check if %s exists in the workspace: %w", o.addIngressFrom, err)
	}
	if !exist {
		return fmt.Errorf("workload %s not found in the workspace", o.addIngressFrom)
	}
	return nil

}

func (o *initStorageOpts) validateServerlessVersion() error {
	for _, valid := range auroraServerlessVersions {
		if o.auroraServerlessVersion == valid {
			return nil
		}
	}
	fmtErrInvalidServerlessVersion := "invalid Aurora Serverless version %s: must be one of %s"
	return fmt.Errorf(fmtErrInvalidServerlessVersion, o.auroraServerlessVersion, prettify(auroraServerlessVersions))
}

// Ask asks for fields that are required but not passed in.
func (o *initStorageOpts) Ask() error {
	if o.addIngressFrom != "" {
		return nil
	}
	if err := o.validateOrAskStorageType(); err != nil {
		return err
	}
	if err := o.askWorkload(); err != nil {
		return err
	}
	// Storage name needs to be asked after workload because for Aurora the default storage name uses the workload name.
	if err := o.validateOrAskStorageName(); err != nil {
		return err
	}
	if err := o.validateOrAskLifecycle(); err != nil {
		return err
	}
	if err := o.validateWorkloadNameWithLifecycle(); err != nil {
		return err
	}
	switch o.storageType {
	case dynamoDBStorageType:
		if err := o.validateOrAskDynamoPartitionKey(); err != nil {
			return err
		}
		if err := o.validateOrAskDynamoSortKey(); err != nil {
			return err
		}
		if err := o.validateOrAskDynamoLSIConfig(); err != nil {
			return err
		}
	case rdsStorageType:
		if err := o.validateOrAskAuroraEngineType(); err != nil {
			return err
		}
		// Ask for initial db name after engine type since the name needs to be validated accordingly.
		if err := o.validateOrAskAuroraInitialDBName(); err != nil {
			return err
		}
	}
	return nil
}

func (o *initStorageOpts) validateOrAskStorageType() error {
	if o.storageType != "" {
		return o.validateStorageType()
	}
	options := []prompt.Option{
		{
			Value:        dynamoDBStorageType,
			FriendlyText: dynamoDBStorageTypeOption,
			Hint:         "NoSQL",
		},
		{
			Value:        s3StorageType,
			FriendlyText: s3StorageTypeOption,
			Hint:         "Objects",
		},
		{
			Value:        rdsStorageType,
			FriendlyText: rdsStorageTypeOption,
			Hint:         "SQL",
		},
	}
	result, err := o.prompt.SelectOption(o.storageTypePrompt(),
		storageInitTypeHelp,
		options,
		prompt.WithFinalMessage("Storage type:"))
	if err != nil {
		return fmt.Errorf("select storage type: %w", err)
	}
	o.storageType = result
	return o.validateStorageType()
}

func (o *initStorageOpts) storageTypePrompt() string {
	if o.workloadName == "" {
		return "What " + color.Emphasize("type") + " of storage would you like to create?"
	}
	return fmt.Sprintf("What "+color.Emphasize("type")+" of storage would you like to associate with %s?", color.HighlightUserInput(o.workloadName))
}

func (o *initStorageOpts) validateStorageType() error {
	if err := validateStorageType(o.storageType, validateStorageTypeOpts{
		ws:           o.ws,
		workloadName: o.workloadName,
	}); err != nil {
		if errors.Is(err, errRDWSNotConnectedToVPC) {
			log.Errorf(`Your %s needs to be connected to a VPC in order to use a %s resource.
You can enable VPC connectivity by updating your manifest with:
%s
`, manifestinfo.RequestDrivenWebServiceType, o.storageType, color.HighlightCodeBlock(`network:
  vpc:
    placement: private`))
		}
		return err
	}
	return nil
}

func (o *initStorageOpts) validateOrAskStorageName() error {
	if o.storageName != "" {
		if err := o.validateStorageName(); err != nil {
			return fmt.Errorf("validate storage name: %w", err)
		}
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
	case rdsStorageType:
		return o.askStorageNameWithDefault(rdsFriendlyText, fmt.Sprintf(fmtRDSStorageNameDefault, o.workloadName), rdsNameValidation)
	}

	name, err := o.prompt.Get(fmt.Sprintf(fmtStorageInitNamePrompt,
		color.HighlightUserInput(friendlyText)),
		storageInitNameHelp,
		validator,
		prompt.WithFinalMessage("Storage resource name:"))
	if err != nil {
		return fmt.Errorf("input storage name: %w", err)
	}
	o.storageName = name
	return nil
}

func (o *initStorageOpts) validateStorageName() error {
	switch o.storageType {
	case dynamoDBStorageType:
		return dynamoTableNameValidation(o.storageName)
	case s3StorageType:
		return s3BucketNameValidation(o.storageName)
	case rdsStorageType:
		return rdsNameValidation(o.storageName)
	default:
		// use dynamo since it's a superset of s3
		return dynamoTableNameValidation(o.storageName)
	}
}

func (o *initStorageOpts) askStorageNameWithDefault(friendlyText, defaultName string, validator func(interface{}) error) error {
	name, err := o.prompt.Get(fmt.Sprintf(fmtStorageInitNamePrompt,
		color.HighlightUserInput(friendlyText)),
		storageInitNameHelp,
		validator,
		prompt.WithFinalMessage("Storage resource name:"),
		prompt.WithDefaultInput(defaultName))

	if err != nil {
		return fmt.Errorf("input storage name: %w", err)
	}
	o.storageName = name
	return nil
}

func (o *initStorageOpts) askWorkload() error {
	if o.workloadName != "" {
		return nil
	}
	if o.lifecycle == lifecycleWorkloadLevel {
		return o.askLocalWorkload()
	}
	workload, err := o.configSel.Workload(storageInitSvcPrompt, "", o.appName)
	if err != nil {
		return fmt.Errorf("select a workload from app %s: %w", o.appName, err)
	}
	o.workloadName = workload
	return nil
}

func (o *initStorageOpts) askLocalWorkload() error {
	workload, err := o.sel.Workload(storageInitSvcPrompt, "")
	if err != nil {
		return fmt.Errorf("retrieve local workload names: %w", err)
	}
	o.workloadName = workload
	return nil
}

func (o *initStorageOpts) validateOrAskLifecycle() error {
	if o.lifecycle != "" {
		return o.validateStorageLifecycle()
	}
	_, err := o.ws.ReadFile(o.ws.WorkloadAddonFileAbsPath(o.workloadName, fmt.Sprintf("%s.yml", o.storageName)))
	if err == nil {
		o.lifecycle = lifecycleWorkloadLevel
		log.Infof("%s %s %s\n",
			color.Emphasize("Lifecycle:"),
			"workload-level",
			color.Faint.Sprintf("(found %s)", o.ws.WorkloadAddonFilePath(o.workloadName, fmt.Sprintf("%s.yml", o.storageName))),
		)
		return nil
	}
	if _, ok := err.(*workspace.ErrFileNotExists); !ok {
		return fmt.Errorf("check if %s addon exists for %s in workspace: %w", o.storageName, o.workloadName, err)
	}
	_, err = o.ws.ReadFile(o.ws.EnvAddonFileAbsPath(fmt.Sprintf("%s.yml", o.storageName)))
	if err == nil {
		o.lifecycle = lifecycleEnvironmentLevel
		log.Infof("%s %s %s\n",
			color.Emphasize("Lifecycle:"),
			"environment-level",
			color.Faint.Sprintf("(found %s)", o.ws.EnvAddonFilePath(fmt.Sprintf("%s.yml", o.storageName))),
		)
		return nil
	}
	if _, ok := err.(*workspace.ErrFileNotExists); !ok {
		return fmt.Errorf("check if %s exists as an environment addon in workspace: %w", o.storageName, err)
	}
	options := []prompt.Option{
		{
			Value:        lifecycleWorkloadLevel,
			FriendlyText: fmt.Sprintf(fmtLifecycleWorkloadFriendlyText, color.HighlightUserInput(o.workloadName)),
		},
		{
			Value:        lifecycleEnvironmentLevel,
			FriendlyText: lifecycleEnvironmentFriendlyText,
		},
	}
	lifecycle, err := o.prompt.SelectOption(
		fmt.Sprintf(fmtStorageInitLifecyclePrompt, o.workloadName),
		"",
		options,
		prompt.WithFinalMessage("Lifecycle: "))
	if err != nil {
		return fmt.Errorf("ask for lifecycle: %w", err)
	}
	o.lifecycle = lifecycle
	return nil
}

func (o *initStorageOpts) validateStorageLifecycle() error {
	for _, valid := range validLifecycleOptions {
		if o.lifecycle == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid lifecycle; must be one of %s", english.OxfordWordSeries(applyAll(validLifecycleOptions, strconv.Quote), "or"))
}

// validateWorkloadNameWithLifecycle requires the workload to be in the workspace if the storage lifecycle is on workload-level.
// Otherwise, it only caches whether the workload is present.
func (o *initStorageOpts) validateWorkloadNameWithLifecycle() error {
	if o.lifecycle == lifecycleEnvironmentLevel {
		hasEnv, err := o.ws.HasEnvironments()
		if err != nil {
			return fmt.Errorf("check if environments directory exists in the workspace: %w", err)
		}
		if !hasEnv {
			return &errNoEnvironmentInWorkspace{}
		}
		return nil
	}
	if o.lifecycle == lifecycleWorkloadLevel {
		exists, err := o.ws.WorkloadExists(o.workloadName)
		if err != nil {
			return fmt.Errorf("check if %s exists in the workspace: %w", o.workloadName, err)
		}
		if !exists {
			return &errWorkloadNotInWorkspace{
				workloadName: o.workloadName,
			}
		}
	}

	return nil
}

func (o *initStorageOpts) validateOrAskDynamoPartitionKey() error {
	if o.partitionKey != "" {
		if err := validateKey(o.partitionKey); err != nil {
			return fmt.Errorf("validate partition key: %w", err)
		}
		return nil
	}
	keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
		color.HighlightUserInput("partition key"),
		color.HighlightUserInput(dynamoDBStorageType),
	)
	key, err := o.prompt.Get(keyPrompt,
		storageInitDDBPartitionKeyHelp,
		dynamoAttributeNameValidation,
		prompt.WithFinalMessage("Partition key:"),
	)
	if err != nil {
		return fmt.Errorf("get DDB partition key: %w", err)
	}

	keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
	keyTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, ddbKeyString)

	keyType, err := o.prompt.SelectOne(keyTypePrompt,
		keyTypeHelp,
		attributeTypes,
		prompt.WithFinalMessage("Partition key datatype:"),
	)
	if err != nil {
		return fmt.Errorf("get DDB partition key datatype: %w", err)
	}

	o.partitionKey = key + ":" + keyType
	return nil
}

func (o *initStorageOpts) validateOrAskDynamoSortKey() error {
	if o.sortKey != "" {
		if err := validateKey(o.sortKey); err != nil {
			return fmt.Errorf("validate sort key: %w", err)
		}
		return nil
	}
	// If the user has not specified a sort key and has specified the --no-sort flag we don't have to demand it of them.
	if o.noSort {
		return nil
	}

	response, err := o.prompt.Confirm(storageInitDDBSortKeyConfirm, storageInitDDBSortKeyHelp, prompt.WithFinalMessage("Sort key?"))
	if err != nil {
		return fmt.Errorf("confirm DDB sort key: %w", err)
	}
	if !response {
		o.noSort = true
		return nil
	}

	keyPrompt := fmt.Sprintf(fmtStorageInitDDBKeyPrompt,
		color.HighlightUserInput("sort key"),
		color.HighlightUserInput(dynamoDBStorageType),
	)
	key, err := o.prompt.Get(keyPrompt,
		storageInitDDBSortKeyHelp,
		dynamoAttributeNameValidation,
		prompt.WithFinalMessage("Sort key:"),
	)
	if err != nil {
		return fmt.Errorf("get DDB sort key: %w", err)
	}
	keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
	keyTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, ddbKeyString)

	keyType, err := o.prompt.SelectOne(keyTypePrompt,
		keyTypeHelp,
		attributeTypes,
		prompt.WithFinalMessage("Sort key datatype:"),
	)
	if err != nil {
		return fmt.Errorf("get DDB sort key datatype: %w", err)
	}
	o.sortKey = key + ":" + keyType
	return nil
}

func (o *initStorageOpts) validateOrAskDynamoLSIConfig() error {
	// LSI has already been specified by flags.
	if len(o.lsiSorts) > 0 {
		return validateLSIs(o.lsiSorts)
	}
	// If --no-lsi has been specified, there is no need to ask for local secondary indices.
	if o.noLSI {
		return nil
	}
	// If --no-sort has been specified, there is no need to ask for local secondary indices.
	if o.noSort {
		o.noLSI = true
		return nil
	}
	lsiTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, color.Emphasize("alternate sort key"))
	lsiTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, "alternate sort key")

	moreLSI, err := o.prompt.Confirm(storageInitDDBLSIPrompt, storageInitDDBLSIHelp, prompt.WithFinalMessage("Additional sort keys?"))
	if err != nil {
		return fmt.Errorf("confirm add alternate sort key: %w", err)
	}
	for {
		if len(o.lsiSorts) > 5 {
			log.Infoln("You may not specify more than 5 alternate sort keys. Continuing...")
			moreLSI = false
		}
		// This will execute last in the loop if moreLSI is set to false by any confirm prompts.
		if !moreLSI {
			o.noLSI = len(o.lsiSorts) == 0
			return nil
		}

		lsiName, err := o.prompt.Get(storageInitDDBLSINamePrompt,
			storageInitDDBLSINameHelp,
			dynamoTableNameValidation,
			prompt.WithFinalMessage("Alternate Sort Key:"),
		)
		if err != nil {
			return fmt.Errorf("get DDB alternate sort key name: %w", err)
		}
		lsiType, err := o.prompt.SelectOne(lsiTypePrompt,
			lsiTypeHelp,
			attributeTypes,
			prompt.WithFinalMessage("Attribute type:"),
		)
		if err != nil {
			return fmt.Errorf("get DDB alternate sort key type: %w", err)
		}

		o.lsiSorts = append(o.lsiSorts, lsiName+":"+lsiType)

		moreLSI, err = o.prompt.Confirm(
			storageInitDDBMoreLSIPrompt,
			storageInitDDBLSIHelp,
			prompt.WithFinalMessage("Additional sort keys?"),
		)
		if err != nil {
			return fmt.Errorf("confirm add alternate sort key: %w", err)
		}
	}
}

func (o *initStorageOpts) validateOrAskAuroraEngineType() error {
	if o.rdsEngine != "" {
		return validateEngine(o.rdsEngine)
	}
	engine, err := o.prompt.SelectOne(storageInitRDSDBEnginePrompt,
		"",
		engineTypes,
		prompt.WithFinalMessage("Database engine:"))
	if err != nil {
		return fmt.Errorf("select database engine: %w", err)
	}
	o.rdsEngine = engine
	return nil
}

func (o *initStorageOpts) validateOrAskAuroraInitialDBName() error {
	var validator func(interface{}) error
	switch o.rdsEngine {
	case engineTypeMySQL:
		validator = validateMySQLDBName
	case engineTypePostgreSQL:
		validator = validatePostgreSQLDBName
	default:
		return errors.New("unknown engine type")
	}

	if o.rdsInitialDBName != "" {
		// The flag input is validated here because it needs engine type to determine which validator to use.
		return validator(o.rdsInitialDBName)
	}

	dbName, err := o.prompt.Get(storageInitRDSInitialDBNamePrompt,
		"",
		validator,
		prompt.WithFinalMessage("Initial database name:"))
	if err != nil {
		return fmt.Errorf("input initial database name: %w", err)
	}
	o.rdsInitialDBName = dbName
	return nil
}

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (o *initStorageOpts) Execute() error {
	o.consumeFlags()
	if err := o.checkWorkloadExists(); err != nil {
		return err
	}
	if err := o.readWorkloadType(); err != nil {
		return err
	}
	addonBlobs, err := o.addonBlobs()
	if err != nil {
		return err
	}
	for _, addon := range addonBlobs {
		path, err := o.ws.Write(addon.blob, addon.path)
		if err == nil {
			log.Successf("Wrote CloudFormation %s at %s\n",
				addon.description,
				color.HighlightResource(displayPath(path)),
			)
			continue
		}
		var errFileExists *workspace.ErrFileExists
		if !errors.As(err, &errFileExists) {
			return err
		}
		log.Successf("CloudFormation %s already exists at %s, skipping writing it.\n",
			addon.description,
			color.HighlightResource(displayPath(addon.path)))
		if addon.description == blobDescriptionParameters {
			log.Infoln(indentBy(color.Faint.Sprintf(addon.recommendedAction()), 2))
		}
	}
	log.Infoln()
	return nil
}

func (o *initStorageOpts) consumeFlags() {
	if o.addIngressFrom == "" {
		return
	}
	o.workloadName = o.addIngressFrom
	o.lifecycle = lifecycleEnvironmentLevel
	if o.storageType == rdsStorageType && o.rdsEngine == "" {
		o.rdsEngine = addon.RDSEngineTypeMySQL
	}
}

func (o *initStorageOpts) checkWorkloadExists() error {
	exist, err := o.ws.WorkloadExists(o.workloadName)
	if err != nil {
		return fmt.Errorf("check if %s exists in the workspace: %w", o.workloadName, err)
	}
	o.workloadExists = exist
	return nil
}

func (o *initStorageOpts) readWorkloadType() error {
	if !o.workloadExists {
		o.workloadType = workloadTypeNonLocal
		return nil
	}
	mft, err := o.ws.ReadWorkloadManifest(o.workloadName)
	if err != nil {
		return fmt.Errorf("read manifest for %s: %w", o.workloadName, err)
	}
	t, err := mft.WorkloadType()
	if err != nil {
		return fmt.Errorf("read 'type' from manifest for %s: %w", o.workloadName, err)
	}
	o.workloadType = t
	return nil
}

type addonBlob struct {
	path        string
	description string
	blob        encoding.BinaryMarshaler
}

func (b *addonBlob) recommendedAction() string {
	data, err := b.blob.MarshalBinary()
	if err != nil {
		return "" // Best effort to read the content in order to make suggestions.
	}
	return fmt.Sprintf("Check that %s has the following snippet:\n%s", displayPath(b.path), color.HighlightCodeBlock(string(data)))
}

func (o *initStorageOpts) addonBlobs() ([]addonBlob, error) {
	type option struct {
		lifecycle   string
		storageType string
	}
	selection := option{o.lifecycle, o.storageType}
	switch selection {
	case option{lifecycleWorkloadLevel, s3StorageType}:
		return o.wkldS3AddonBlobs()
	case option{lifecycleWorkloadLevel, dynamoDBStorageType}:
		return o.wkldDDBAddonBlobs()
	case option{lifecycleWorkloadLevel, rdsStorageType}:
		return o.wkldRDSAddonBlobs()
	case option{lifecycleEnvironmentLevel, s3StorageType}:
		return o.envS3AddonBlobs()
	case option{lifecycleEnvironmentLevel, dynamoDBStorageType}:
		return o.envDDBAddonBlobs()
	case option{lifecycleEnvironmentLevel, rdsStorageType}:
		return o.envRDSAddonBlobs()
	}
	return nil, fmt.Errorf("storage type %s is not supported yet", o.storageType)
}

func (o *initStorageOpts) wkldDDBAddonBlobs() ([]addonBlob, error) {
	props, err := o.ddbProps()
	if err != nil {
		return nil, err
	}
	return []addonBlob{
		{
			path:        o.ws.WorkloadAddonFilePath(o.workloadName, fmt.Sprintf("%s.yml", o.storageName)),
			description: blobDescriptionTemplate,
			blob:        addon.WorkloadDDBTemplate(props),
		},
	}, nil
}

func (o *initStorageOpts) envDDBAddonBlobs() ([]addonBlob, error) {
	ingressBlob := addonBlob{
		path:        o.ws.WorkloadAddonFilePath(o.workloadName, fmt.Sprintf("%s-access-policy.yml", o.storageName)),
		description: blobDescriptionTemplate,
		blob: addon.EnvDDBAccessPolicyTemplate(&addon.AccessPolicyProps{
			Name: o.storageName,
		}),
	}
	if o.addIngressFrom != "" {
		return []addonBlob{ingressBlob}, nil
	}
	props, err := o.ddbProps()
	if err != nil {
		return nil, err
	}
	tmplBlob := addonBlob{
		path:        o.ws.EnvAddonFilePath(fmt.Sprintf("%s.yml", o.storageName)),
		description: blobDescriptionTemplate,
		blob:        addon.EnvDDBTemplate(props),
	}
	if !o.workloadExists {
		return []addonBlob{tmplBlob}, nil
	}
	return []addonBlob{tmplBlob, ingressBlob}, nil
}

func (o *initStorageOpts) ddbProps() (*addon.DynamoDBProps, error) {
	props := addon.DynamoDBProps{
		StorageProps: &addon.StorageProps{
			Name: o.storageName,
		},
	}

	if err := props.BuildPartitionKey(o.partitionKey); err != nil {
		return nil, err
	}

	hasSortKey, err := props.BuildSortKey(o.noSort, o.sortKey)
	if err != nil {
		return nil, err
	}

	if hasSortKey {
		_, err := props.BuildLocalSecondaryIndex(o.noLSI, o.lsiSorts)
		if err != nil {
			return nil, err
		}
	}
	return &props, nil
}

func (o *initStorageOpts) wkldS3AddonBlobs() ([]addonBlob, error) {
	return []addonBlob{
		{
			path:        o.ws.WorkloadAddonFilePath(o.workloadName, fmt.Sprintf("%s.yml", o.storageName)),
			description: blobDescriptionTemplate,
			blob:        addon.WorkloadS3Template(o.s3Props()),
		},
	}, nil
}

func (o *initStorageOpts) envS3AddonBlobs() ([]addonBlob, error) {
	ingressBlob := addonBlob{
		path:        o.ws.WorkloadAddonFilePath(o.workloadName, fmt.Sprintf("%s-access-policy.yml", o.storageName)),
		description: blobDescriptionTemplate,
		blob: addon.EnvS3AccessPolicyTemplate(&addon.AccessPolicyProps{
			Name: o.storageName,
		}),
	}
	if o.addIngressFrom != "" {
		return []addonBlob{ingressBlob}, nil
	}
	tmplBlob := addonBlob{
		path:        o.ws.EnvAddonFilePath(fmt.Sprintf("%s.yml", o.storageName)),
		description: blobDescriptionTemplate,
		blob:        addon.EnvS3Template(o.s3Props()),
	}
	if !o.workloadExists {
		return []addonBlob{tmplBlob}, nil
	}
	return []addonBlob{tmplBlob, ingressBlob}, nil
}

func (o *initStorageOpts) s3Props() *addon.S3Props {
	return &addon.S3Props{
		StorageProps: &addon.StorageProps{
			Name: o.storageName,
		},
	}
}

func (o *initStorageOpts) wkldRDSAddonBlobs() ([]addonBlob, error) {
	props, err := o.rdsProps()
	if err != nil {
		return nil, err
	}
	var tmplBlob encoding.BinaryMarshaler
	switch {
	case o.auroraServerlessVersion == auroraServerlessVersionV1 && o.workloadType == manifestinfo.RequestDrivenWebServiceType:
		tmplBlob = addon.RDWSServerlessV1Template(props)
	case o.auroraServerlessVersion == auroraServerlessVersionV1 && o.workloadType != manifestinfo.RequestDrivenWebServiceType:
		tmplBlob = addon.WorkloadServerlessV1Template(props)
	case o.auroraServerlessVersion == auroraServerlessVersionV2 && o.workloadType == manifestinfo.RequestDrivenWebServiceType:
		tmplBlob = addon.RDWSServerlessV2Template(props)
	case o.auroraServerlessVersion == auroraServerlessVersionV2 && o.workloadType != manifestinfo.RequestDrivenWebServiceType:
		tmplBlob = addon.WorkloadServerlessV2Template(props)
	default:
		return nil, fmt.Errorf("unknown combination of serverless version %q and workload type %q", o.auroraServerlessVersion, o.workloadType)
	}
	blobs := []addonBlob{{
		path:        o.ws.WorkloadAddonFilePath(o.workloadName, fmt.Sprintf("%s.yml", o.storageName)),
		description: blobDescriptionTemplate,
		blob:        tmplBlob,
	}}
	if o.workloadType != manifestinfo.RequestDrivenWebServiceType {
		return blobs, nil
	}
	return append(blobs, addonBlob{
		path:        o.ws.WorkloadAddonFilePath(o.workloadName, workspace.AddonsParametersFileName),
		description: blobDescriptionParameters,
		blob:        addon.RDWSParamsForRDS(),
	}), nil
}

func (o *initStorageOpts) envRDSAddonBlobs() ([]addonBlob, error) {
	if o.workloadType == manifestinfo.RequestDrivenWebServiceType {
		return o.envRDSForRDWSAddonBlobs()
	}
	if o.addIngressFrom != "" {
		return nil, nil
	}
	props, err := o.rdsProps()
	if err != nil {
		return nil, err
	}
	tmplBlob := addonBlob{
		path:        o.ws.EnvAddonFilePath(fmt.Sprintf("%s.yml", o.storageName)),
		description: blobDescriptionTemplate,
		blob:        addon.EnvServerlessTemplate(props),
	}
	paramBlob := addonBlob{
		path:        o.ws.EnvAddonFilePath(workspace.AddonsParametersFileName),
		description: blobDescriptionParameters,
		blob:        addon.EnvParamsForRDS(),
	}
	return []addonBlob{tmplBlob, paramBlob}, nil
}

func (o *initStorageOpts) envRDSForRDWSAddonBlobs() ([]addonBlob, error) {
	rdwsIngressTmplBlob := addonBlob{
		path:        o.ws.WorkloadAddonFilePath(o.workloadName, fmt.Sprintf("%s-ingress.yml", o.storageName)),
		description: blobDescriptionTemplate,
		blob: addon.EnvServerlessRDWSIngressTemplate(addon.RDSIngressProps{
			ClusterName: o.storageName,
			Engine:      o.rdsEngine,
		}),
	}
	rdwsIngressParamBlob := addonBlob{
		path:        o.ws.WorkloadAddonFilePath(o.workloadName, workspace.AddonsParametersFileName),
		description: blobDescriptionParameters,
		blob:        addon.RDWSParamsForEnvRDS(),
	}
	if o.addIngressFrom != "" {
		return []addonBlob{rdwsIngressTmplBlob, rdwsIngressParamBlob}, nil
	}
	props, err := o.rdsProps()
	if err != nil {
		return nil, err
	}
	tmplBlob := addonBlob{
		path:        o.ws.EnvAddonFilePath(fmt.Sprintf("%s.yml", o.storageName)),
		description: blobDescriptionTemplate,
		blob:        addon.EnvServerlessForRDWSTemplate(props),
	}
	paramBlob := addonBlob{
		path:        o.ws.EnvAddonFilePath(workspace.AddonsParametersFileName),
		description: blobDescriptionParameters,
		blob:        addon.EnvParamsForRDS(),
	}
	if o.workloadExists {
		return []addonBlob{tmplBlob, paramBlob, rdwsIngressTmplBlob, rdwsIngressParamBlob}, nil
	}
	return []addonBlob{tmplBlob, paramBlob}, nil
}

func (o *initStorageOpts) rdsProps() (addon.RDSProps, error) {
	envs, err := o.environmentNames()
	if err != nil {
		return addon.RDSProps{}, err
	}
	return addon.RDSProps{
		ClusterName:    o.storageName,
		Engine:         o.rdsEngine,
		InitialDBName:  o.rdsInitialDBName,
		ParameterGroup: o.rdsParameterGroup,
		Envs:           envs,
	}, nil
}

func (o *initStorageOpts) environmentNames() ([]string, error) {
	var envNames []string
	envs, err := o.store.ListEnvironments(o.appName)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	for _, env := range envs {
		envNames = append(envNames, env.Name)
	}
	return envNames, nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *initStorageOpts) RecommendActions() error {
	switch o.lifecycle {
	case lifecycleWorkloadLevel:
		logRecommendedActions(o.actionsForWorkloadStorage())
	case lifecycleEnvironmentLevel:
		logRecommendedActions(o.actionsForEnvStorage())
	}
	return nil
}

func (o *initStorageOpts) actionsForWorkloadStorage() []string {
	var (
		retrieveEnvVarCode string
		newVar             string
	)
	switch o.storageType {
	case dynamoDBStorageType, s3StorageType:
		newVar = template.ToSnakeCaseFunc(template.EnvVarNameFunc(o.storageName))
		retrieveEnvVarCode = fmt.Sprintf("const storageName = process.env.%s", newVar)
	case rdsStorageType:
		newVar = template.ToSnakeCaseFunc(template.EnvVarSecretFunc(o.storageName))
		retrieveEnvVarCode = fmt.Sprintf("const {username, host, dbname, password, port} = JSON.parse(process.env.%s)", newVar)
		if o.workloadType == manifestinfo.RequestDrivenWebServiceType {
			newVar = fmt.Sprintf("%s_ARN", newVar)
			retrieveEnvVarCode = fmt.Sprintf(`const AWS = require('aws-sdk');
const client = new AWS.SecretsManager({
    region: process.env.AWS_DEFAULT_REGION,
});
const dbSecret = await client.getSecretValue({SecretId: process.env.%s}).promise();
const {username, host, dbname, password, port} = JSON.parse(dbSecret.SecretString);`, newVar)
		}
	}

	actionRetrieveEnvVar := fmt.Sprintf(
		`Update %s's code to leverage the injected environment variable %s.
For example, in JavaScript you can write:
%s`,
		o.workloadName,
		newVar,
		color.HighlightCodeBlock(retrieveEnvVarCode))

	deployCmd := fmt.Sprintf("copilot deploy --name %s", o.workloadName)
	actionDeploy := fmt.Sprintf("Run %s to deploy your storage resources.", color.HighlightCode(deployCmd))
	return []string{
		actionRetrieveEnvVar,
		actionDeploy,
	}
}

func (o *initStorageOpts) actionsForEnvStorage() []string {
	envDeployAction := fmt.Sprintf("Run %s to deploy your environment storage resources.", color.HighlightCode("copilot env deploy"))
	svcMftAction := fmt.Sprintf("Update the manifest for your %q workload:\n%s", o.workloadName, color.HighlightCodeBlock(o.manifestSuggestion()))
	svcDeployAction := fmt.Sprintf("Run %s to deploy the workload so that %s has access to %s storage.",
		color.HighlightCode(fmt.Sprintf("copilot svc deploy --name %s", o.workloadName)),
		color.HighlightUserInput(o.workloadName),
		color.HighlightUserInput(o.storageName))
	addIngressAction := fmt.Sprintf("From the workspace where %s is, run:\n%s",
		color.HighlightUserInput(o.workloadName),
		color.HighlightCodeBlock(o.addIngressSuggestion()))
	if envAndWkldWritten := o.addIngressFrom == "" && o.workloadExists; envAndWkldWritten {
		return []string{envDeployAction, svcMftAction, svcDeployAction}
	}
	if onlyEnv := !o.workloadExists; onlyEnv {
		return []string{envDeployAction, addIngressAction}
	}
	if onlyWkld := o.addIngressFrom != ""; onlyWkld {
		return []string{svcMftAction, svcDeployAction}
	}
	return nil
}

func (o *initStorageOpts) manifestSuggestion() string {
	logicalIDSafeStorageName := template.StripNonAlphaNumFunc(o.storageName)
	switch {
	case o.storageType == s3StorageType:
		return fmt.Sprintf(`variables:
  BUCKET_NAME:
    from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-%sBucketName`, logicalIDSafeStorageName)
	case o.storageType == dynamoDBStorageType:
		return fmt.Sprintf(`variables:
  DB_NAME:
    from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-%sTableName`, logicalIDSafeStorageName)
	case o.storageType == rdsStorageType && o.workloadType == manifestinfo.RequestDrivenWebServiceType:
		return fmt.Sprintf(`secrets:
  DB_SECRET:
    from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-%sAuroraSecret`, logicalIDSafeStorageName)
	case o.storageType == rdsStorageType && o.workloadType != manifestinfo.RequestDrivenWebServiceType:
		return fmt.Sprintf(`network:
  vpc:
    security_groups:
      - from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-%sSecurityGroup
secrets:
  DB_SECRET:
    from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-%sAuroraSecret`,
			logicalIDSafeStorageName, logicalIDSafeStorageName)
	}
	return ""
}

func (o *initStorageOpts) addIngressSuggestion() string {
	return fmt.Sprintf(`copilot storage init -n %s \
--storage-type %s \
--add-ingress-from %s`, o.storageName, o.storageType, o.workloadName)
}

type errWorkloadNotInWorkspace struct {
	workloadName string
}

func (e *errWorkloadNotInWorkspace) Error() string {
	return fmt.Sprintf("workload %s not found in the workspace", e.workloadName)
}

// RecommendActions suggests actions to fix the error.
func (e *errWorkloadNotInWorkspace) RecommendActions() string {
	return fmt.Sprintf("Run %s in the workspace where %s is instead.", color.HighlightCode("copilot storage init"), color.HighlightUserInput(e.workloadName))
}

type errNoEnvironmentInWorkspace struct{}

func (e *errNoEnvironmentInWorkspace) Error() string {
	return "environments are not managed in the workspace"
}

// RecommendActions suggests actions to fix the error.
func (e *errNoEnvironmentInWorkspace) RecommendActions() string {
	return fmt.Sprintf("Run %s in the workspace where environments are managed instaed.", color.HighlightCode("copilot storage init"))
}

// buildStorageInitCmd builds the command and adds it to the CLI.
func buildStorageInitCmd() *cobra.Command {
	vars := initStorageVars{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new AWS CloudFormation template for a storage resource.",
		Long: `Creates a new AWS CloudFormation template for a storage resource.
Storage resources are addons, either for a workload or the environments.`,
		Example: `
  Create an S3 bucket named "my-bucket" attached to the "frontend" service.
  /code $ copilot storage init -n my-bucket -t S3 -w frontend -l workload
  Create an environment S3 bucket fronted by the "api" service.
  /code $ copilot storage init -n my-bucket -t S3 -w api -l environment
  Create a DynamoDB table with a sort key.
  /code $ copilot storage init -n my-table -t DynamoDB -w frontend --partition-key Email:S --sort-key UserId:N --no-lsi
  Create an RDS Aurora Serverless v2 cluster using PostgreSQL.
  /code $ copilot storage init -n my-cluster -t Aurora -w frontend --engine PostgreSQL --initial-db testdb`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newStorageInitOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.storageName, nameFlag, nameFlagShort, "", storageFlagDescription)
	cmd.Flags().StringVarP(&vars.storageType, storageTypeFlag, typeFlagShort, "", storageTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.workloadName, workloadFlag, workloadFlagShort, "", storageWorkloadFlagDescription)
	cmd.Flags().StringVarP(&vars.lifecycle, storageLifecycleFlag, storageLifecycleShort, "", storageLifecycleFlagDescription)
	cmd.Flags().StringVarP(&vars.addIngressFrom, storageAddIngressFromFlag, "", "", storageAddIngressFromFlagDescription)

	cmd.Flags().StringVar(&vars.partitionKey, storagePartitionKeyFlag, "", storagePartitionKeyFlagDescription)
	cmd.Flags().StringVar(&vars.sortKey, storageSortKeyFlag, "", storageSortKeyFlagDescription)
	cmd.Flags().StringArrayVar(&vars.lsiSorts, storageLSIConfigFlag, []string{}, storageLSIConfigFlagDescription)
	cmd.Flags().BoolVar(&vars.noLSI, storageNoLSIFlag, false, storageNoLSIFlagDescription)
	cmd.Flags().BoolVar(&vars.noSort, storageNoSortFlag, false, storageNoSortFlagDescription)

	cmd.Flags().StringVar(&vars.auroraServerlessVersion, storageAuroraServerlessVersionFlag, defaultAuroraServerlessVersion, storageAuroraServerlessVersionFlagDescription)
	cmd.Flags().StringVar(&vars.rdsEngine, storageRDSEngineFlag, "", storageRDSEngineFlagDescription)
	cmd.Flags().StringVar(&vars.rdsInitialDBName, storageRDSInitialDBFlag, "", storageRDSInitialDBFlagDescription)
	cmd.Flags().StringVar(&vars.rdsParameterGroup, storageRDSParameterGroupFlag, "", storageRDSParameterGroupFlagDescription)

	ddbFlags := []string{storagePartitionKeyFlag, storageSortKeyFlag, storageNoSortFlag, storageLSIConfigFlag, storageNoLSIFlag}
	rdsFlags := []string{storageAuroraServerlessVersionFlag, storageRDSEngineFlag, storageRDSInitialDBFlag, storageRDSParameterGroupFlag}
	for _, f := range append(ddbFlags, storageAuroraServerlessVersionFlag, storageRDSInitialDBFlag, storageRDSParameterGroupFlag) {
		cmd.MarkFlagsMutuallyExclusive(storageAddIngressFromFlag, f)
	}
	requiredFlags := pflag.NewFlagSet("Required", pflag.ContinueOnError)
	requiredFlags.AddFlag(cmd.Flags().Lookup(nameFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(storageTypeFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(workloadFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(storageLifecycleFlag))

	ddbFlagSet := pflag.NewFlagSet("DynamoDB", pflag.ContinueOnError)
	for _, f := range ddbFlags {
		ddbFlagSet.AddFlag(cmd.Flags().Lookup(f))
	}
	auroraFlagSet := pflag.NewFlagSet("Aurora Serverless", pflag.ContinueOnError)
	for _, f := range rdsFlags {
		auroraFlagSet.AddFlag(cmd.Flags().Lookup(f))
	}

	optionalFlagSet := pflag.NewFlagSet("Optional", pflag.ContinueOnError)
	optionalFlagSet.AddFlag(cmd.Flags().Lookup(storageAddIngressFromFlag))

	cmd.Annotations = map[string]string{
		// The order of the sections we want to display.
		"sections":          `Required,DynamoDB,Aurora Serverless,Optional`,
		"Required":          requiredFlags.FlagUsages(),
		"DynamoDB":          ddbFlagSet.FlagUsages(),
		"Aurora Serverless": auroraFlagSet.FlagUsages(),
		"Optional":          optionalFlagSet.FlagUsages(),
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
