// Copyright Amazon.com, Inc or its affiliates. All rights reserved.

package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	dynamoDBStorageType = "Dynamo DB"
	s3StorageType       = "S3 Bucket"
)

var storageTypes = []string{
	dynamoDBStorageType,
	s3StorageType,
}

var (
	fmtStorageInitTypePrompt = "What " + color.Emphasize("type") + " of storage would you like to associate with %s?"
	storageInitTypeHelp      = "The type of storage you'd like to add to your project. You can choose between key-value (DynamoDB) content (S3) storage types."
	fmtStorageInitNamePrompt = "What would you like to " + color.Emphasize("name") + " this %s?"
	storageInitNameHelp      = "The name of this storage resource. You can use the following characters: a-zA-Z0-9-_"
	storageInitSvcPrompt     = "Which " + color.Emphasize("service") + " would you like to associate with this storage resource?"
	storageInitSvcHelp       = "The service you'd like to have access to this storage resource. We'll deploy the Cloudformation for the storage when you run `app deploy`."
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

	fs        afero.Fs
	ws        wsSvcReader
	store     store
	appGetter store

	proj *config.Application
}

func newStorageInitOpts(vars initStorageVars) (*initStorageOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to application datastore: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}

	return &initStorageOpts{
		initStorageVars: vars,

		fs:        &afero.Afero{Fs: afero.NewOsFs()},
		store:     store,
		appGetter: store,
		ws:        ws,
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
		return fmt.Errorf("retrieve local app names: %w", err)
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
		return fmt.Errorf("list services in the workspace: %w", err)
	}
	for _, name := range names {
		if o.StorageSvc == name {
			return nil
		}
	}
	return fmt.Errorf("service %s not found in the workspace", o.StorageSvc)
}

func BuildStorageInitCmd() *cobra.Command {
	vars := initStorageVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Hidden: false, //TODO remove when ready for production
		Use:    "init",
		Short:  "Creates a new storage table in an environment.",
		Example: `
  Create a "my-table" DynamoDB table in the "test" environment.
  /code $ copilot storage init --name my-table --storage-type dynamo-db --app frontend`,
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
			// if err := opts.Execute(); err != nil {
			// 	return err
			// }
			log.Infoln("Recommended follow-up actions:")
			// for _, followup := range opts.RecommendedActions() {
			// 	log.Infof("- %s\n", followup)
			// }
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.StorageName, nameFlag, nameFlagShort, "", storageFlagDescription)
	cmd.Flags().StringVar(&vars.StorageType, storageTypeFlag, "", storageTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.StorageSvc, appFlag, appFlagShort, "", storageServiceFlagDescription)

	return cmd
}
