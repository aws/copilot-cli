// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImportVPCConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		inVPCID     string
		inPublicID  []string
		inPrivateID []string

		wantedEmpty bool
	}{
		"non empty if has vpc id": {
			inVPCID: "mockID",

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
			inVPCID:     "",

			wantedEmpty: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			config := ImportVPCConfig{
				ID:               tc.inVPCID,
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

func TestAdjustVPCConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		inVPCCIDR     string
		inPublicCIDR  []string
		inPrivateCIDR []string

		wantedEmpty bool
	}{
		"non empty if has vpc cidr": {
			inVPCCIDR: "mockCIDR",

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
			inVPCCIDR:     EmptyIPNetString,

			wantedEmpty: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			config := AdjustVPCConfig{
				CIDR:               tc.inVPCCIDR,
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
