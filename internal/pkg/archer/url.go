// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Secretsmanager can manage secrets in an underlying secret management store
type URLManager interface {
	URLCreator
	URLDeleter
}

// URLDeleter adds a record set to Route53
type URLCreator interface {
	CreateCNAME(source, target string) error
}

// URLDeleter deletes a record set from Route53
type URLDeleter interface {
	DeleteCNAME(source, target string) error
}
