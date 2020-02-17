// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Secretsmanager can create secrets in an underlying secret management store
type SecretsManager interface {
	SecretCreator
	SecretDeleter
}

// SecretCreator creates a secretin the underlying secret management store
type SecretCreator interface {
	CreateSecret(secretName, secretString string) (string, error)
}

// SecretDeleter deletes secret from the underlying secret management store
type SecretDeleter interface {
	DeleteSecret(secretName string) error
}
