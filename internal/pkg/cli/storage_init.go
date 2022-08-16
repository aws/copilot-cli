// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
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

var optionToStorageType = map[string]string{
	dynamoDBStorageTypeOption: dynamoDBStorageType,
	s3StorageTypeOption:       s3StorageType,
	rdsStorageTypeOption:      rdsStorageType,
}

var storageTypeOptions = map[string]prompt.Option{
	dynamoDBStorageType: {
		Value: dynamoDBStorageTypeOption,
		Hint:  "NoSQL",
	},
	s3StorageType: {
		Value: s3StorageTypeOption,
		Hint:  "Objects",
	},
	rdsStorageType: {
		Value: rdsStorageTypeOption,
		Hint:  "SQL",
	},
}

const (
	s3BucketFriendlyText      = "S3 Bucket"
	dynamoDBTableFriendlyText = "DynamoDB Table"
	rdsFriendlyText           = "Database Cluster"
)

// General-purpose prompts, collected for all storage resources.
var (
	fmtStorageInitTypePrompt = "What " + color.Emphasize("type") + " of storage would you like to associate with %s?"
	storageInitTypeHelp      = `The type of storage you'd like to add to your workload. 
DynamoDB is a key-value and document database that delivers single-digit millisecond performance at any scale.
S3 is a web object store built to store and retrieve any amount of data from anywhere on the Internet.
Aurora Serverless is an on-demand autoscaling configuration for Amazon Aurora, a MySQL and PostgreSQL-compatible relational database.
`

	fmtStorageInitNamePrompt = "What would you like to " + color.Emphasize("name") + " this %s?"
	storageInitNameHelp      = "The name of this storage resource. You can use the following characters: a-zA-Z0-9-_"

	storageInitSvcPrompt = "Which " + color.Emphasize("workload") + " would you like to associate with this storage resource?"
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
	fmtRDSStorageNameDefault = "%s-cluster"

	engineTypeMySQL      = "MySQL"
	engineTypePostgreSQL = "PostgreSQL"
)

var engineTypes = []string{
	engineTypeMySQL,
	engineTypePostgreSQL,
}

var errUnavailableAddonParams = errors.New("addon does not require parameters")

type initStorageVars struct {
	storageType  string
	storageName  string
	workloadName string

	// Dynamo DB specific values collected via flags or prompts
	partitionKey string
	sortKey      string
	lsiSorts     []string // lsi sort keys collected as "name:T" where T is one of [SNB]
	noLSI        bool
	noSort       bool

	// RDS Aurora Serverless specific values collected via flags or prompts
	rdsEngine         string
	rdsParameterGroup string
	rdsInitialDBName  string
}

type initStorageOpts struct {
	initStorageVars
	appName string

	fs    afero.Fs
	ws    wsAddonManager
	store store

	sel    wsSelector
	prompt prompter

	// Cached data.
	workloadType string
}

func newStorageInitOpts(vars initStorageVars) (*initStorageOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("storage init"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	return &initStorageOpts{
		initStorageVars: vars,
		appName:         tryReadingAppName(),

		fs:     &afero.Afero{Fs: afero.NewOsFs()},
		store:  store,
		ws:     ws,
		sel:    selector.NewLocalWorkloadSelector(prompter, store, ws),
		prompt: prompter,
	}, nil
}

func (o *initStorageOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.workloadName != "" {
		if err := o.validateWorkloadName(); err != nil {
			return err
		}
	}
	if o.storageType != "" {
		if err := o.validateStorageType(); err != nil {
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
		case rdsStorageType:
			err = rdsNameValidation(o.storageName)
		default:
			// use dynamo since it's a superset of s3
			err = dynamoTableNameValidation(o.storageName)
		}
		if err != nil {
			return err
		}
	}
	if err := o.validateDDB(); err != nil {
		return err
	}

	if o.rdsEngine != "" {
		if err := validateEngine(o.rdsEngine); err != nil {
			return err
		}
	}
	return nil
}

func (o *initStorageOpts) validateDDB() error {
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
	// --no-lsi and --lsi are mutually exclusive.
	if o.noLSI && len(o.lsiSorts) != 0 {
		return fmt.Errorf("validate LSI configuration: cannot specify --no-lsi and --lsi options at once")
	}

	// --no-sort and --lsi are mutually exclusive.
	if o.noSort && len(o.lsiSorts) != 0 {
		return fmt.Errorf("validate LSI configuration: cannot specify --no-sort and --lsi options at once")
	}
	if len(o.lsiSorts) != 0 {
		if err := validateLSIs(o.lsiSorts); err != nil {
			return err
		}
	}
	return nil
}

func (o *initStorageOpts) Ask() error {
	if err := o.askStorageWl(); err != nil {
		return err
	}
	if err := o.askStorageType(); err != nil {
		return err
	}

	// Storage name needs to be asked after workload because for Aurora the default storage name uses the workload name.
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
		if err := o.askDynamoLSIConfig(); err != nil {
			return err
		}
	case rdsStorageType:
		if err := o.askAuroraEngineType(); err != nil {
			return err
		}
		// Ask for initial db name after engine type since the name needs to be validated accordingly.
		if err := o.askAuroraInitialDBName(); err != nil {
			return err
		}
	}
	return nil
}

func (o *initStorageOpts) askStorageType() error {
	if o.storageType != "" {
		return o.validateStorageType()
	}
	var options []prompt.Option
	for _, st := range storageTypes {
		options = append(options, storageTypeOptions[st])
	}
	storageTypeOption, err := o.prompt.SelectOption(fmt.Sprintf(
		fmtStorageInitTypePrompt, color.HighlightUserInput(o.workloadName)),
		storageInitTypeHelp,
		options,
		prompt.WithFinalMessage("Storage type:"))
	if err != nil {
		return fmt.Errorf("select storage type: %w", err)
	}
	o.storageType = optionToStorageType[storageTypeOption]
	return o.validateStorageType()
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
`, manifest.RequestDrivenWebServiceType, o.storageType, color.HighlightCodeBlock(`network:
  vpc:
    placement: private`))
		}
		return err
	}
	return nil
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

func (o *initStorageOpts) askStorageWl() error {
	if o.workloadName != "" {
		return nil
	}
	workload, err := o.sel.Workload(storageInitSvcPrompt, "")
	if err != nil {
		return fmt.Errorf("retrieve local workload names: %w", err)
	}
	o.workloadName = workload
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

func (o *initStorageOpts) askDynamoSortKey() error {
	if o.sortKey != "" {
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

func (o *initStorageOpts) askDynamoLSIConfig() error {
	// LSI has already been specified by flags.
	if len(o.lsiSorts) > 0 {
		return nil
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

func (o *initStorageOpts) askAuroraEngineType() error {
	if o.rdsEngine != "" {
		return nil
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

func (o *initStorageOpts) askAuroraInitialDBName() error {
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

func (o *initStorageOpts) validateWorkloadName() error {
	names, err := o.ws.ListWorkloads()
	if err != nil {
		return fmt.Errorf("retrieve local workload names: %w", err)
	}
	for _, name := range names {
		if o.workloadName == name {
			return nil
		}
	}
	return fmt.Errorf("workload %s not found in the workspace", o.workloadName)
}

func (o *initStorageOpts) Execute() error {
	if err := o.readWorkloadType(); err != nil {
		return err
	}

	addonBlobs, err := o.addonBlobs()
	if err != nil {
		return err
	}
	for _, addon := range addonBlobs {
		path, err := o.ws.WriteAddon(addon.blob, o.workloadName, addon.name)
		if err != nil {
			e, ok := err.(*workspace.ErrFileExists)
			if !ok {
				return err
			}
			return fmt.Errorf("addon file already exists: %w", e)
		}
		path = displayPath(path)
		log.Successf("Wrote CloudFormation %s at %s\n",
			addon.description,
			color.HighlightResource(path),
		)
	}
	log.Infoln()
	return nil
}

type addonBlob struct {
	name        string
	description string
	blob        encoding.BinaryMarshaler
}

func (o *initStorageOpts) addonBlobs() ([]addonBlob, error) {
	templateBlob, err := o.newAddonTemplate()
	if err != nil {
		return nil, err
	}
	blobs := []addonBlob{
		{
			name:        o.storageName,
			description: "template",
			blob:        templateBlob,
		},
	}
	paramsBlob, err := o.newAddonParams()
	if err != nil {
		if errors.Is(err, errUnavailableAddonParams) { // The addon does not need any parameters.
			return blobs, nil
		}
		return nil, err
	}
	return append(blobs, addonBlob{
		name:        "addons.parameters",
		description: "parameters",
		blob:        paramsBlob,
	}), nil
}

func (o *initStorageOpts) newAddonTemplate() (encoding.BinaryMarshaler, error) {
	var templateBlob encoding.BinaryMarshaler
	err := fmt.Errorf("storage type %s doesn't have a CF template", o.storageType)
	switch o.storageType {
	case dynamoDBStorageType:
		templateBlob, err = o.newDDBTemplate()
	case s3StorageType:
		templateBlob, err = o.newS3Template()
	case rdsStorageType:
		templateBlob, err = o.newRDSTemplate()
	}
	if err != nil {
		return nil, err
	}
	return templateBlob, nil
}

func (o *initStorageOpts) newAddonParams() (encoding.BinaryMarshaler, error) {
	if o.storageType != rdsStorageType {
		return nil, errUnavailableAddonParams
	}
	if o.workloadType != manifest.RequestDrivenWebServiceType {
		return nil, errUnavailableAddonParams
	}
	return addon.NewRDSParams(), nil
}

func (o *initStorageOpts) newDDBTemplate() (*addon.DynamoDBTemplate, error) {
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

	return addon.NewDDBTemplate(&props), nil
}

func (o *initStorageOpts) newS3Template() (*addon.S3Template, error) {
	props := &addon.S3Props{
		StorageProps: &addon.StorageProps{
			Name: o.storageName,
		},
	}
	return addon.NewS3Template(props), nil
}

func (o *initStorageOpts) newRDSTemplate() (*addon.RDSTemplate, error) {
	var engine string
	switch o.rdsEngine {
	case engineTypeMySQL:
		engine = addon.RDSEngineTypeMySQL
	case engineTypePostgreSQL:
		engine = addon.RDSEngineTypePostgreSQL
	default:
		return nil, errors.New("unknown engine type")
	}

	envs, err := o.environmentNames()
	if err != nil {
		return nil, err
	}

	return addon.NewRDSTemplate(addon.RDSProps{
		ClusterName:    o.storageName,
		Engine:         engine,
		InitialDBName:  o.rdsInitialDBName,
		ParameterGroup: o.rdsParameterGroup,
		Envs:           envs,
		WorkloadType:   o.workloadType,
	}), nil
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

func (o *initStorageOpts) readWorkloadType() error {
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

func (o *initStorageOpts) RecommendActions() error {
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
		if o.workloadType == manifest.RequestDrivenWebServiceType {
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
	logRecommendedActions([]string{
		actionRetrieveEnvVar,
		actionDeploy,
	})
	return nil
}

// buildStorageInitCmd builds the command and adds it to the CLI.
func buildStorageInitCmd() *cobra.Command {
	vars := initStorageVars{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new AWS CloudFormation template for a storage resource.",
		Long: `Creates a new AWS CloudFormation template for a storage resource.
Storage resources are stored in the Copilot addons directory (e.g. ./copilot/frontend/addons) for a given workload and deployed to your environments when you run ` + color.HighlightCode("copilot deploy") + `. 
Resource names are injected into your containers as environment variables for easy access.`,
		Example: `
  Create an S3 bucket named "my-bucket" attached to the "frontend" service.
  /code $ copilot storage init -n my-bucket -t S3 -w frontend
  Create a basic DynamoDB table named "my-table" attached to the "frontend" service with a sort key specified.
  /code $ copilot storage init -n my-table -t DynamoDB -w frontend --partition-key Email:S --sort-key UserId:N --no-lsi
  Create a DynamoDB table with multiple alternate sort keys.
  /code $ copilot storage init -n my-table -t DynamoDB -w frontend --partition-key Email:S --sort-key UserId:N --lsi Points:N --lsi Goodness:N
  Create an RDS Aurora Serverless cluster using PostgreSQL as the database engine.
  /code $ copilot storage init -n my-cluster -t Aurora -w frontend --engine PostgreSQL`,
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

	cmd.Flags().StringVar(&vars.partitionKey, storagePartitionKeyFlag, "", storagePartitionKeyFlagDescription)
	cmd.Flags().StringVar(&vars.sortKey, storageSortKeyFlag, "", storageSortKeyFlagDescription)
	cmd.Flags().StringArrayVar(&vars.lsiSorts, storageLSIConfigFlag, []string{}, storageLSIConfigFlagDescription)
	cmd.Flags().BoolVar(&vars.noLSI, storageNoLSIFlag, false, storageNoLSIFlagDescription)
	cmd.Flags().BoolVar(&vars.noSort, storageNoSortFlag, false, storageNoSortFlagDescription)

	cmd.Flags().StringVar(&vars.rdsEngine, storageRDSEngineFlag, "", storageRDSEngineFlagDescription)
	cmd.Flags().StringVar(&vars.rdsInitialDBName, storageRDSInitialDBFlag, "", storageRDSInitialDBFlagDescription)
	cmd.Flags().StringVar(&vars.rdsParameterGroup, storageRDSParameterGroupFlag, "", storageRDSParameterGroupFlagDescription)

	requiredFlags := pflag.NewFlagSet("Required", pflag.ContinueOnError)
	requiredFlags.AddFlag(cmd.Flags().Lookup(nameFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(storageTypeFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(workloadFlag))

	ddbFlags := pflag.NewFlagSet("DynamoDB", pflag.ContinueOnError)
	ddbFlags.AddFlag(cmd.Flags().Lookup(storagePartitionKeyFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageSortKeyFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageNoSortFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageLSIConfigFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageNoLSIFlag))

	auroraFlags := pflag.NewFlagSet("Aurora Serverless", pflag.ContinueOnError)
	auroraFlags.AddFlag(cmd.Flags().Lookup(storageRDSEngineFlag))
	auroraFlags.AddFlag(cmd.Flags().Lookup(storageRDSInitialDBFlag))
	auroraFlags.AddFlag(cmd.Flags().Lookup(storageRDSParameterGroupFlag))

	cmd.Annotations = map[string]string{
		// The order of the sections we want to display.
		"sections":          `Required,DynamoDB,Aurora Serverless`,
		"Required":          requiredFlags.FlagUsages(),
		"DynamoDB":          ddbFlags.FlagUsages(),
		"Aurora Serverless": auroraFlags.FlagUsages(),
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
