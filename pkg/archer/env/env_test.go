// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvironment_Marshal(t *testing.T) {
	// GIVEN
	environ := &Environment{
		Name:      "chicken",
		Region:    "us-west-2",
		AccountID: "11111111111",
	}
	want := `{"name":"chicken","region":"us-west-2","accountID":"11111111111"}`

	// WHEN
	got, _ := environ.Marshal()
	require.Equal(t, want, got)
}
