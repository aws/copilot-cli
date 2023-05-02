// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package delete

// NoOpDeleter does nothing.
type NoOpDeleter struct{}

// CleanResources returns nil.
func (n *NoOpDeleter) CleanResources(app, env, wkld string) error {
	return nil
}
