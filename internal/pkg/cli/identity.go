// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import "github.com/aws/copilot-cli/internal/pkg/aws/identity"

type identityService interface {
	Get() (identity.Caller, error)
}
