// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package route53

import "errors"

// ErrNoDefaultCluster occurs when the default cluster is not found.
var ErrDomainNotExist = errors.New("domain does not exist")
