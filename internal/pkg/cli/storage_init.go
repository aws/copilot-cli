// Copyright Amazon.com, Inc or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
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
We'll deploy the resources for the storage when you run 'svc deploy'.`
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

	storageInitDDBLSIPrompt = "Would you like to add any alternate sort keys to this table?"
	storageInitDDBLSIHelp   = `Alternate sort keys create Local Secondary Indexes, which allow you to sort the table using the same 
partition key but a different sort key. You may specify up to 5 alternate sort keys.`

	storageInitDDBMoreLSIPrompt = "Would you like to add more alternate sort keys to this table?"

	storageInitDDBLSINamePrompt = "What would you like to name this " + color.Emphasize("alternate sort key") + "?"
	storageInitDDBLSINameHelp   = "You can use the the characters [a-zA-Z0-9.-_]"

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

type attribute struct {
	name     string
	dataType string
}

type initStorageVars struct {
	*GlobalOpts
	storageType string
	storageName string
	storageSvc  string

	// Dynamo DB specific values collected via flags or prompts
	partitionKey string
	sortKey      string
	lsiSorts     []string // lsi sort keys collected as "name:T" where T is one of [SNB]
	noLSI        bool
	noSort       bool
}

type initStorageOpts struct {
	initStorageVars

	fs    afero.Fs
	ws    wsAddonManager
	store store

	app *config.Application
	sel wsSelector
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
		sel:   selector.NewWorkspaceSelect(vars.prompt, store, ws),
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
		storageTypes,
		prompt.WithFinalMessage("Storage type:"))
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
		validator,
		prompt.WithFinalMessage("Storage resource name:"))

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
		prompt.WithFinalMessage("Partition key:"),
	)
	if err != nil {
		return fmt.Errorf("get DDB partition key: %w", err)
	}

	keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
	keyTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, ddbKeyString)

	keyType, err := o.prompt.SelectOne(keyTypePrompt,
		keyTypeHelp,
		attributeTypesLong,
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
		prompt.WithFinalMessage("Sort key:"),
	)
	if err != nil {
		return fmt.Errorf("get DDB sort key: %w", err)
	}
	keyTypePrompt := fmt.Sprintf(fmtStorageInitDDBKeyTypePrompt, ddbKeyString)
	keyTypeHelp := fmt.Sprintf(fmtStorageInitDDBKeyTypeHelp, ddbKeyString)

	keyType, err := o.prompt.SelectOne(keyTypePrompt,
		keyTypeHelp,
		attributeTypesLong,
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
			attributeTypesLong,
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

	return o.createAddon()
}

var regexpMatchAttribute = regexp.MustCompile("^(\\S+):([sbnSBN])")

// getAttFromKey parses the DDB type and name out of keys specified in the form "Email:S"
func getAttrFromKey(input string) (attribute, error) {
	attrs := regexpMatchAttribute.FindStringSubmatch(input)
	if len(attrs) == 0 {
		return attribute{}, fmt.Errorf("parse attribute from key: %s", input)
	}
	return attribute{
		name:     attrs[1],
		dataType: strings.ToUpper(attrs[2]),
	}, nil
}

func (o *initStorageOpts) createAddon() error {
	addonCf, err := o.newAddon()
	if err != nil {
		return err
	}

	addonPath, err := o.ws.WriteAddon(addonCf, o.storageSvc, o.storageName)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return err
		}
		return fmt.Errorf("addon already exists: %w", e)
	}
	addonPath, err = relPath(addonPath)
	if err != nil {
		return err
	}

	addonMsgFmt := "Wrote CloudFormation template for %[1]s %[2]s at %[3]s\n"
	var addonFriendlyText string
	switch o.storageType {
	case dynamoDBStorageType:
		addonFriendlyText = dynamoDBTableFriendlyText
	case s3StorageType:
		addonFriendlyText = s3BucketFriendlyText
	default:
		return fmt.Errorf(fmtErrInvalidStorageType, o.storageType, prettify(storageTypes))
	}
	log.Successf(addonMsgFmt,
		color.Emphasize(addonFriendlyText),
		color.HighlightUserInput(o.storageName),
		color.HighlightResource(addonPath),
	)
	log.Infoln(color.Help(`The Cloudformation template is a nested stack which fully describes your resource,
the IAM policy necessary for an ECS task to access that resource, and outputs
which are injected as environment variables into the Copilot service this addon
is associated with.`))
	log.Infoln()

	return nil
}
func (o *initStorageOpts) newAddon() (encoding.BinaryMarshaler, error) {
	switch o.storageType {
	case dynamoDBStorageType:
		return o.newDynamoDBAddon()
	case s3StorageType:
		return o.newS3Addon()
	default:
		return nil, fmt.Errorf("storage type %s doesn't have a CF template", o.storageType)
	}
}

func newDDBAttribute(input string) (*addon.DDBAttribute, error) {
	attr, err := getAttrFromKey(input)
	if err != nil {
		return nil, err
	}
	return &addon.DDBAttribute{
		Name:     &attr.name,
		DataType: &attr.dataType,
	}, nil
}

func newLSI(partitionKey string, lsis []string) ([]addon.DDBLocalSecondaryIndex, error) {
	var output []addon.DDBLocalSecondaryIndex
	for _, lsi := range lsis {
		lsiAttr, err := getAttrFromKey(lsi)
		if err != nil {
			return nil, err
		}
		output = append(output, addon.DDBLocalSecondaryIndex{
			PartitionKey: &partitionKey,
			SortKey:      &lsiAttr.name,
			Name:         &lsiAttr.name,
		})
	}
	return output, nil
}

func (o *initStorageOpts) newDynamoDBAddon() (*addon.DynamoDB, error) {
	props := addon.DynamoDBProps{}

	var attributes []addon.DDBAttribute
	partKey, err := newDDBAttribute(o.partitionKey)
	if err != nil {
		return nil, err
	}
	props.PartitionKey = partKey.Name
	attributes = append(attributes, *partKey)
	if !o.noSort {
		sortKey, err := newDDBAttribute(o.sortKey)
		if err != nil {
			return nil, err
		}
		attributes = append(attributes, *sortKey)
		props.SortKey = sortKey.Name
	}
	for _, att := range o.lsiSorts {
		currAtt, err := newDDBAttribute(att)
		if err != nil {
			return nil, err
		}
		attributes = append(attributes, *currAtt)
	}
	props.Attributes = attributes
	// only configure LSI if we haven't specified the --no-lsi flag.
	props.HasLSI = false
	if !o.noLSI {
		props.HasLSI = true
		lsiConfig, err := newLSI(
			*partKey.Name,
			o.lsiSorts,
		)
		if err != nil {
			return nil, err
		}
		props.LSIs = lsiConfig
	}

	props.StorageProps = &addon.StorageProps{
		Name: o.storageName,
	}
	return addon.NewDynamoDB(&props), nil
}

func (o *initStorageOpts) newS3Addon() (*addon.S3, error) {
	props := &addon.S3Props{
		StorageProps: &addon.StorageProps{
			Name: o.storageName,
		},
	}
	return addon.NewS3(props), nil
}

func (o *initStorageOpts) RecommendedActions() []string {

	newVar := template.ToSnakeCaseFunc(template.EnvVarNameFunc(o.storageName))

	svcDeployCmd := fmt.Sprintf("copilot svc deploy --name %s", o.storageSvc)

	return []string{
		fmt.Sprintf("Update your service code to leverage the injected environment variable %s", color.HighlightCode(newVar)),
		fmt.Sprintf("Run %s to deploy your storage resources to your environments.", color.HighlightCode(svcDeployCmd)),
	}
}

// BuildStorageInitCmd builds the command and adds it to the CLI.
func BuildStorageInitCmd() *cobra.Command {
	vars := initStorageVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Hidden: true, //TODO remove when ready for production
		Use:    "init",
		Short:  "Creates a new storage table in an environment.",
		Example: `
  Create a "my-bucket" S3 bucket bucket attached to the "frontend" service.
  /code $ copilot storage init -n my-bucket -t S3 -s frontend
  Create a basic DynamoDB table named "my-table" attached to the "frontend" service.
  /code $ copilot storage init -n my-table -t DynamoDB -s frontend --partition-key Email:S --sort-key UserId:N --no-lsi
  Create a DynamoDB table with multiple alternate sort keys.
  /code $ copilot storage init -n my-table -t DynamoDB -s frontend --partition-key Email:S --sort-key UserId:N --lsi Points:N --lsi Goodness:N`,
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
			for _, followup := range opts.RecommendedActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.storageName, nameFlag, nameFlagShort, "", storageFlagDescription)
	cmd.Flags().StringVarP(&vars.storageType, storageTypeFlag, svcTypeFlagShort, "", storageTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.storageSvc, svcFlag, svcFlagShort, "", storageServiceFlagDescription)

	cmd.Flags().StringVar(&vars.partitionKey, storagePartitionKeyFlag, "", storagePartitionKeyFlagDescription)
	cmd.Flags().StringVar(&vars.sortKey, storageSortKeyFlag, "", storageSortKeyFlagDescription)
	cmd.Flags().StringArrayVar(&vars.lsiSorts, storageLSIConfigFlag, []string{}, storageLSIConfigFlagDescription)
	cmd.Flags().BoolVar(&vars.noLSI, storageNoLSIFlag, false, storageNoLSIFlagDescription)
	cmd.Flags().BoolVar(&vars.noSort, storageNoSortFlag, false, storageNoSortFlagDescription)

	requiredFlags := pflag.NewFlagSet("Required", pflag.ContinueOnError)
	requiredFlags.AddFlag(cmd.Flags().Lookup(nameFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(storageTypeFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(svcFlag))

	ddbFlags := pflag.NewFlagSet("DynamoDB", pflag.ContinueOnError)
	ddbFlags.AddFlag(cmd.Flags().Lookup(storagePartitionKeyFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageSortKeyFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageNoSortFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageLSIConfigFlag))
	ddbFlags.AddFlag(cmd.Flags().Lookup(storageNoLSIFlag))
	cmd.Annotations = map[string]string{
		// The order of the sections we want to display.
		"sections": `Required,DynamoDB`,
		"Required": requiredFlags.FlagUsages(),
		"DynamoDB": ddbFlags.FlagUsages(),
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
