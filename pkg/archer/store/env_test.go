// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvironment_Marshal(t *testing.T) {
	// GIVEN
	environ := &Environment{
		Name:      "test",
		Region:    "us-west-2",
		AccountID: "11111111111",
		Project:   "chicken",
		Prod:      false,
	}
	want := `{"project":"chicken","name":"test","region":"us-west-2","accountID":"11111111111","prod":false}`

	// WHEN
	got, _ := environ.Marshal()
	require.Equal(t, want, got)
}
