// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImportVpcConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		inVpcID     string
		inPublicID  []string
		inPrivateID []string

		wantedEmpty bool
	}{
		"non empty if has vpc id": {
			inVpcID: "mockID",

			wantedEmpty: false,
		},
		"non empty if has public subnets id": {
			inPublicID: []string{"mockID"},

			wantedEmpty: false,
		},
		"non empty if has private id": {
			inPrivateID: []string{"mockID"},

			wantedEmpty: false,
		},
		"empty": {
			inPrivateID: []string{},
			inPublicID:  []string{},
			inVpcID:     "",

			wantedEmpty: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			config := ImportVpcConfig{
				ID:               tc.inVpcID,
				PrivateSubnetIDs: tc.inPrivateID,
				PublicSubnetIDs:  tc.inPublicID,
			}

			// WHEN
			empty := config.IsEmpty()

			// THEN
			require.Equal(t, empty, tc.wantedEmpty)
		})
	}
}

func TestAdjustVpcConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		inVpcCIDR     string
		inPublicCIDR  []string
		inPrivateCIDR []string

		wantedEmpty bool
	}{
		"non empty if has vpc cidr": {
			inVpcCIDR: "mockCIDR",

			wantedEmpty: false,
		},
		"non empty if has public subnets cidr": {
			inPublicCIDR: []string{"mockCIDR"},

			wantedEmpty: false,
		},
		"non empty if has private cidr": {
			inPrivateCIDR: []string{"mockCIDR"},

			wantedEmpty: false,
		},
		"empty": {
			inPrivateCIDR: []string{},
			inPublicCIDR:  []string{},
			inVpcCIDR:     EmptyIPNetString,

			wantedEmpty: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			config := AdjustVpcConfig{
				CIDR:               tc.inVpcCIDR,
				PublicSubnetCIDRs:  tc.inPublicCIDR,
				PrivateSubnetCIDRs: tc.inPrivateCIDR,
			}

			// WHEN
			empty := config.IsEmpty()

			// THEN
			require.Equal(t, empty, tc.wantedEmpty)
		})
	}
}
