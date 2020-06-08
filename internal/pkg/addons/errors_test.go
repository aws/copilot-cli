// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestErrMetadataKeyAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var err error
	err = &errMetadataKeyAlreadyExists{
		errKeyAlreadyExists: &errKeyAlreadyExists{
			Key: "Version",
			First: &yaml.Node{
				Tag:   "!!str",
				Kind:  yaml.ScalarNode,
				Value: "1",
			},
			Second: &yaml.Node{
				Tag:   "!!str",
				Kind:  yaml.ScalarNode,
				Value: "2",
			},
		},
	}
	type humanError interface {
		HumanError() string
	}

	// THEN
	herr, ok := err.(humanError)
	require.True(t, ok, "expected errMetadataKeyAlreadyExists to implement humanError")
	require.Equal(t, fmt.Sprintf(`The "Metadata" key %s exists with two different values under addons:
%s
and
%s`,
		color.HighlightCode("Version"),
		color.HighlightCode(`"1"`),
		color.HighlightCode(`"2"`)),
		herr.HumanError())
}
