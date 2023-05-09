// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

type versionGetterDouble struct {
	VersionFn func() (string, error)
}

func (d *versionGetterDouble) Version() (string, error) {
	return d.VersionFn()
}
