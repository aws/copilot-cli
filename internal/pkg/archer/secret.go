// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Secretsmanager can manage secrets in an underlying secret management store
type SecretsManager interface {
	SecretCreator
	SecretDeleter
}

// SecretCreator creates a secret in the underlying secret management store
type SecretCreator interface {
	CreateSecret(secretName, secretString string) (string, error)
}

// SecretDeleter deletes a secret in the underlying secret management store
type SecretDeleter interface {
	DeleteSecret(secretName string) error
}
