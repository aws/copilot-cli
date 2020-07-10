// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecs

import "errors"

// ErrNoDefaultCluster occurs when the default cluster is not found.
var ErrNoDefaultCluster = errors.New("default cluster does not exist")

