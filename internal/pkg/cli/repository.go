// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"

	"github.com/google/go-github/github"
)

type repository interface {
	List(ctx context.Context, user string, opt *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error)
	ListBranches(ctx context.Context, owner string, repo string, opt *github.ListOptions) ([]*github.Branch, *github.Response, error)
}
