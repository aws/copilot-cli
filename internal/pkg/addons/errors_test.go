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

func TestErrKeyAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var err error
	err = &errKeyAlreadyExists{
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
	}
	type humanError interface {
		HumanError() string
	}

	// THEN
	herr, ok := err.(humanError)
	require.True(t, ok, "expected errKeyAlreadyExists to implement humanError")
	require.Equal(t, fmt.Sprintf(`%s exists with two different values under addons:
%s
and
%s`,
		color.HighlightCode("Version"),
		color.HighlightCode(`"1"`),
		color.HighlightCode(`"2"`)),
		herr.HumanError())
}

func TestErrMetadataKeyAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var err error
	err = &errMetadataAlreadyExists{}
	type humanError interface {
		HumanError() string
	}

	// THEN
	_, ok := err.(humanError)
	require.True(t, ok, "expected errMetadataAlreadyExists to implement humanError")
}

func TestErrParameterAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var err error
	err = &errParameterAlreadyExists{}
	type humanError interface {
		HumanError() string
	}

	// THEN
	_, ok := err.(humanError)
	require.True(t, ok, "expected errParameterAlreadyExists to implement humanError")
}

func TestErrMappingAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var err error
	err = &errMappingAlreadyExists{}
	type humanError interface {
		HumanError() string
	}

	// THEN
	_, ok := err.(humanError)
	require.True(t, ok, "expected errMappingAlreadyExists to implement humanError")
}

func TestErrConditionAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var err error
	err = &errConditionAlreadyExists{}
	type humanError interface {
		HumanError() string
	}

	// THEN
	_, ok := err.(humanError)
	require.True(t, ok, "expected errConditionAlreadyExists to implement humanError")
}

func TestErrResourceAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var err error
	err = &errResourceAlreadyExists{}
	type humanError interface {
		HumanError() string
	}

	// THEN
	_, ok := err.(humanError)
	require.True(t, ok, "expected errResourceAlreadyExists to implement humanError")
}
