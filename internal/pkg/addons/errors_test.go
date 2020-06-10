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
	err = &errMetadataAlreadyExists{
		&errKeyAlreadyExists{
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
	require.True(t, ok, "expected errMetadataAlreadyExists to implement humanError")
	require.Equal(t, fmt.Sprintf(`The Metadata key %s exists with two different values under addons:
%s
and
%s`,
		color.HighlightCode("Version"),
		color.HighlightCode(`"1"`),
		color.HighlightCode(`"2"`)),
		herr.HumanError())
}

func TestErrParameterAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var first yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`
Name:
  Type: String
  Description: The name of the service, job, or workflow being deployed.`), &first), "unmarshal first")

	var second yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`
Name:
  Type: String
  Description: The name of the service, job, or workflow being deployed.
  Default: api # "Default" doesn't exist in first.yaml, therefore it should error.`), &second), "unmarshal second")

	var err error
	err = &errParameterAlreadyExists{
		&errKeyAlreadyExists{
			Key:    "Name",
			First:  &first,
			Second: &second,
		},
	}
	type humanError interface {
		HumanError() string
	}

	// THEN
	herr, ok := err.(humanError)
	require.True(t, ok, "expected errParameterAlreadyExists to implement humanError")
	require.Equal(t, fmt.Sprintf(`The Parameter logical ID %s exists with two different values under addons:
%s
and
%s`,
		color.HighlightCode("Name"),
		color.HighlightCode(`Name:
    Type: String
    Description: The name of the service, job, or workflow being deployed.`),
		color.HighlightCode(`Name:
    Type: String
    Description: The name of the service, job, or workflow being deployed.
    Default: api # "Default" doesn't exist in first.yaml, therefore it should error.`)),
		herr.HumanError())
}

func TestErrMappingAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var first yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`
test:
  WCU: 5`), &first), "unmarshal first")

	var second yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`
test:
  RCU: 5`), &second), "unmarshal second")

	var err error
	err = &errMappingAlreadyExists{
		&errKeyAlreadyExists{
			Key:    "MyMapping.test",
			First:  &first,
			Second: &second,
		},
	}
	type humanError interface {
		HumanError() string
	}

	// THEN
	herr, ok := err.(humanError)
	require.True(t, ok, "expected errMappingAlreadyExists to implement humanError")
	require.Equal(t, fmt.Sprintf(`The Mapping %s exists with two different values under addons:
%s
and
%s`,
		color.HighlightCode("MyMapping.test"),
		color.HighlightCode(`test:
    WCU: 5`),
		color.HighlightCode(`test:
    RCU: 5`)),
		herr.HumanError())
}

func TestErrConditionAlreadyExists_HumanError(t *testing.T) {
	// GIVEN
	var err error
	err = &errConditionAlreadyExists{
		&errKeyAlreadyExists{
			Key: "IsProd",
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
	require.True(t, ok, "expected errConditionAlreadyExists to implement humanError")
	require.Equal(t, fmt.Sprintf(`The Condition %s exists with two different values under addons:
%s
and
%s`,
		color.HighlightCode("IsProd"),
		color.HighlightCode(`"1"`),
		color.HighlightCode(`"2"`)),
		herr.HumanError())
}
