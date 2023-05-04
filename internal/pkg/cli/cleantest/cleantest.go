// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// cleantest provides stubs for cli.wkldCleaner.
package cleantest

import "errors"

// Succeeds stubs cli.wkldCleaner and simulates success.
type Succeeds struct{}

// Clean succeeds.
func (*Succeeds) Clean() error {
	return nil
}

// Fails stubs cli.wkldCleaner and simulates failure.
type Fails struct{}

// Clean fails.
func (*Fails) Clean() error {
	return errors.New("an error")
}
