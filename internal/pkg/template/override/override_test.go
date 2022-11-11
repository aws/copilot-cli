// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func newTaskDefPropertyRule(rule Rule) Rule {
	return Rule{
		Path:  fmt.Sprintf("Resources.TaskDefinition.Properties.%s", rule.Path),
		Value: rule.Value,
	}
}

func requiresCompatibilitiesRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "RequiresCompatibilities[-]",
		Value: yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   nodeTagStr,
			Value: "EC2",
		},
	})
}

func linuxParametersCapabilitiesRule() Rule {
	node1 := yaml.Node{
		Kind:  yaml.ScalarNode,
		Style: yaml.DoubleQuotedStyle,
		Tag:   nodeTagStr,
		Value: "AUDIT_CONTROL",
	}
	node2 := yaml.Node{
		Kind:  yaml.ScalarNode,
		Style: yaml.DoubleQuotedStyle,
		Tag:   nodeTagStr,
		Value: "AUDIT_WRITE",
	}
	return newTaskDefPropertyRule(Rule{
		Path: "ContainerDefinitions[0].LinuxParameters.Capabilities.Add",
		Value: yaml.Node{
			Kind:    yaml.SequenceNode,
			Style:   yaml.FlowStyle,
			Tag:     nodeTagSeq,
			Content: []*yaml.Node{&node1, &node2},
		},
	})
}

func linuxParametersCapabilitiesInitProcessEnabledRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "ContainerDefinitions[0].LinuxParameters.Capabilities.InitProcessEnabled",
		Value: yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   nodeTagBool,
			Value: "true",
		},
	})
}

func ulimitsRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "ContainerDefinitions[0].Ulimits[-].HardLimit",
		Value: yaml.Node{
			Kind:  yaml.ScalarNode,
			Style: yaml.TaggedStyle,
			Tag:   "!Ref",
			Value: "ParamName",
		},
	})
}

func logGroupRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "ContainerDefinitions[0].LogConfiguration.Options.awslogs-group",
		Value: yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "/copilot/${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}",
		},
	})
}

func exposeExtraPortRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "ContainerDefinitions[0].PortMappings[-].ContainerPort",
		Value: yaml.Node{
			Kind:  8,
			Tag:   nodeTagInt,
			Value: "5000",
		},
	})
}

func referBadSeqIndexRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "ContainerDefinitions[0].PortMappings[1].ContainerPort",
		Value: yaml.Node{
			Kind:  8,
			Tag:   nodeTagInt,
			Value: "5000",
		},
	})
}

func referBadSeqIndexWithNoKeyRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "ContainerDefinitions[0].VolumesFrom[1].SourceContainer",
		Value: yaml.Node{
			Kind:  8,
			Tag:   nodeTagStr,
			Value: "foo",
		},
	})
}

func Test_CloudFormationTemplate(t *testing.T) {
	testCases := map[string]struct {
		inRules       []Rule
		inTplFileName string

		wantedError       error
		wantedTplFileName string
	}{
		"invalid CFN template": {
			inTplFileName: "empty.yml",
			wantedError:   fmt.Errorf("cannot apply override rule on empty YAML template"),
		},
		"error when referring to bad sequence index": {
			inTplFileName: "backend_svc.yml",
			inRules: []Rule{
				referBadSeqIndexRule(),
			},
			wantedError: fmt.Errorf("cannot specify PortMappings[1] because the current length is 1. Use [%s] to append to the sequence instead", seqAppendToLastSymbol),
		},
		"error when referring to bad sequence index when sequence key doesn't exist": {
			inTplFileName: "backend_svc.yml",
			inRules: []Rule{
				referBadSeqIndexWithNoKeyRule(),
			},

			wantedError: fmt.Errorf("cannot specify VolumesFrom[1] because VolumesFrom does not exist. Use VolumesFrom[%s] to append to the sequence instead", seqAppendToLastSymbol),
		},
		"success with ulimits": {
			inTplFileName: "backend_svc.yml",
			inRules: []Rule{
				ulimitsRule(),
			},
			wantedTplFileName: "ulimits.yml",
		},
		"success with different log group name": {
			inTplFileName: "backend_svc.yml",
			inRules: []Rule{
				logGroupRule(),
			},
			wantedTplFileName: "loggroup.yml",
		},
		"success with extra port": {
			inTplFileName: "backend_svc.yml",
			inRules: []Rule{
				exposeExtraPortRule(),
			},
			wantedTplFileName: "extra_port.yml",
		},
		"success with linux parameters": {
			inTplFileName: "backend_svc.yml",
			inRules: []Rule{
				linuxParametersCapabilitiesRule(),
				linuxParametersCapabilitiesInitProcessEnabledRule(),
			},
			wantedTplFileName: "linux_parameters.yml",
		},
		"success with requires compatibilities": {
			inTplFileName: "backend_svc.yml",
			inRules: []Rule{
				requiresCompatibilitiesRule(),
			},
			wantedTplFileName: "requires_compatibilities.yml",
		},
		"success with multiple override rules": {
			inTplFileName: "backend_svc.yml",
			inRules: []Rule{
				ulimitsRule(),
				exposeExtraPortRule(),
				linuxParametersCapabilitiesRule(),
				linuxParametersCapabilitiesInitProcessEnabledRule(),
				requiresCompatibilitiesRule(),
			},
			wantedTplFileName: "multiple_overrides.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			in, err := os.ReadFile(filepath.Join("testdata", "original", tc.inTplFileName))
			require.NoError(t, err)

			got, gotErr := CloudFormationTemplate(tc.inRules, in)
			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				wantedContent, err := os.ReadFile(filepath.Join("testdata", "outputs", tc.wantedTplFileName))
				require.NoError(t, err)

				require.NoError(t, gotErr)
				require.Equal(t, string(wantedContent), string(got))
			}
		})
	}
}
