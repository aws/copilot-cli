// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

// Noop represents an Overrider that does not do any transformations.
type Noop struct{}

// Override does nothing.
func (no *Noop) Override(body []byte) ([]byte, error) {
	return body, nil
}
