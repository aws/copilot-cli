// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// package delete contains common functions for deleting resources
// created through copilot.
package delete

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
)

type imageRemover interface {
	ClearRepository(repo string) error
}

type regionalSessionProvider interface {
	DefaultWithRegion(region string) (*session.Session, error)
}

type ECREmptier struct {
	SessionProvider regionalSessionProvider

	newImageRemover func(*session.Session) imageRemover // for testing
}

func (e *ECREmptier) defaultNewImageRemover(sess *session.Session) imageRemover {
	return ecr.New(sess)
}

func (e *ECREmptier) EmptyRepo(repo string, regions map[string]struct{}) error {
	if e.newImageRemover == nil {
		e.newImageRemover = e.defaultNewImageRemover
	}

	for region := range regions {
		sess, err := e.SessionProvider.DefaultWithRegion(region)
		if err != nil {
			return err
		}

		client := e.newImageRemover(sess)
		if err := client.ClearRepository(repo); err != nil {
			return err
		}
	}

	return nil
}
