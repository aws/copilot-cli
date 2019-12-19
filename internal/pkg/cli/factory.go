// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
)

type optsFactory struct{}

func (f *optsFactory) CreateInitAppOpts() (*initAppOpts, error) {
	store, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to metadata datastore: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}

	sess, err := session.Default()
	if err != nil {
		return nil, err
	}
	return &initAppOpts{
		AppType:        viper.GetString(appTypeFlag),
		AppName:        viper.GetString(nameFlag),
		DockerfilePath: viper.GetString(dockerFileFlag),
		GlobalOpts:     NewGlobalOpts(),

		fs:             &afero.Afero{Fs: afero.NewOsFs()},
		appStore:       store,
		projGetter:     store,
		manifestWriter: ws,
		projDeployer:   cloudformation.New(sess),
		prog:           termprogress.NewSpinner(),
	}, nil
}
