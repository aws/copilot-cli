// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const testContent = `Resources:
  TaskDefinition:
    Metadata:
      'aws:copilot:description': 'An ECS task definition to group your containers and run them on ECS'
    Type: AWS::ECS::TaskDefinition
    DependsOn: LogGroup
    Properties:
      Family: !Join ['', [!Ref AppName, '-', !Ref EnvName, '-', !Ref WorkloadName]]
      NetworkMode: awsvpc
      RequiresCompatibilities:
        - FARGATE
      Cpu: !Ref TaskCPU
      Memory: !Ref TaskMemory
      ExecutionRoleArn: !Ref ExecutionRole
      TaskRoleArn: !Ref TaskRole
      ContainerDefinitions:
        - Name: !Ref WorkloadName
          Image: !Ref ContainerImage
          
          # We pipe certain environment variables directly into the task definition.
          # This lets customers have access to, for example, their LB endpoint - which they'd
          # have no way of otherwise determining.
          Environment:
          - Name: COPILOT_APPLICATION_NAME
            Value: !Sub '${AppName}'
          - Name: COPILOT_SERVICE_DISCOVERY_ENDPOINT
            Value: test.demo.local
          - Name: COPILOT_ENVIRONMENT_NAME
            Value: !Sub '${EnvName}'
          - Name: COPILOT_SERVICE_NAME
            Value: !Sub '${WorkloadName}'
          - Name: COPILOT_LB_DNS
            Value: !GetAtt EnvControllerAction.PublicLoadBalancerDNSName
          
          LogConfiguration:
            LogDriver: awslogs
            Options:
              awslogs-region: !Ref AWS::Region
              awslogs-group: !Ref LogGroup
              awslogs-stream-prefix: copilot
          
          PortMappings:
            - ContainerPort: !Ref ContainerPort
`

const wantedOverriddenTemplate = `Resources:
  TaskDefinition:
    Metadata:
      'aws:copilot:description': 'An ECS task definition to group your containers and run them on ECS'
    Type: AWS::ECS::TaskDefinition
    DependsOn: LogGroup
    Properties:
      Family: !Join ['', [!Ref AppName, '-', !Ref EnvName, '-', !Ref WorkloadName]]
      NetworkMode: awsvpc
      RequiresCompatibilities:
        - FARGATE
        - EC2
      Cpu: !Ref TaskCPU
      Memory: !Ref TaskMemory
      ExecutionRoleArn: !Ref ExecutionRole
      TaskRoleArn: !Ref TaskRole
      ContainerDefinitions:
        - Name: !Ref WorkloadName
          Image: !Ref ContainerImage
          # We pipe certain environment variables directly into the task definition.
          # This lets customers have access to, for example, their LB endpoint - which they'd
          # have no way of otherwise determining.
          Environment:
            - Name: COPILOT_APPLICATION_NAME
              Value: !Sub '${AppName}'
            - Name: COPILOT_SERVICE_DISCOVERY_ENDPOINT
              Value: test.demo.local
            - Name: COPILOT_ENVIRONMENT_NAME
              Value: !Sub '${EnvName}'
            - Name: COPILOT_SERVICE_NAME
              Value: !Sub '${WorkloadName}'
            - Name: COPILOT_LB_DNS
              Value: !GetAtt EnvControllerAction.PublicLoadBalancerDNSName
          LogConfiguration:
            LogDriver: awslogs
            Options:
              awslogs-region: !Ref AWS::Region
              awslogs-group: !Ref LogGroup
              awslogs-stream-prefix: copilot
          PortMappings:
            - ContainerPort: !Ref ContainerPort
            - ContainerPort: 5000
          Ulimits:
            - HardLimit: !Ref ParamName
          LinuxParameters:
            Capabilities:
              Add: ["AUDIT_CONTROL", "AUDIT_WRITE"]
              InitProcessEnabled: true
`

func newTaskDefPropertyNode(nextNode nodeUpserter) nodeUpserter {
	end := &mapUpsertNode{
		upsertNode: upsertNode{
			key:  "Properties",
			next: nextNode,
		},
	}
	taskDefNode := &mapUpsertNode{
		upsertNode: upsertNode{
			key:  "TaskDefinition",
			next: end,
		},
	}
	head := &mapUpsertNode{
		upsertNode: upsertNode{
			key:  "Resources",
			next: taskDefNode,
		},
	}
	return head
}

func newTaskDefPropertyRule(rule Rule) Rule {
	return Rule{
		Path:  fmt.Sprintf("Resources.TaskDefinition.Properties.%s", rule.Path),
		Value: rule.Value,
	}
}

func requiresCompatibilitiesNode() nodeUpserter {
	var node yaml.Node
	_ = yaml.Unmarshal([]byte(`RequiresCompatibilities[-]: EC2`), &node)

	node1 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:           "RequiresCompatibilities",
			valueToInsert: node.Content[0].Content[1],
		},
		appendToLast: true,
	}
	return newTaskDefPropertyNode(node1)
}

func requiresCompatibilitiesRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "RequiresCompatibilities[-]",
		Value: &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   nodeTagStr,
			Value: "EC2",
		},
	})
}

func linuxParametersCapabilitiesNode() nodeUpserter {
	var node yaml.Node
	_ = yaml.Unmarshal([]byte(`ContainerDefinitions[0].LinuxParameters.Capabilities.Add: ["AUDIT_CONTROL", "AUDIT_WRITE"]`), &node)

	node2 := &mapUpsertNode{
		upsertNode: upsertNode{
			key:           "Add",
			valueToInsert: node.Content[0].Content[1],
		},
	}
	node1 := &mapUpsertNode{
		upsertNode: upsertNode{
			key:  "Capabilities",
			next: node2,
		},
	}
	return newLinuxParametersNode(node1)
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
		Value: &yaml.Node{
			Kind:    yaml.SequenceNode,
			Style:   yaml.FlowStyle,
			Tag:     nodeTagSeq,
			Content: []*yaml.Node{&node1, &node2},
		},
	})
}

func linuxParametersCapabilitiesInitProcessEnabledNode() nodeUpserter {
	node2 := &mapUpsertNode{
		upsertNode: upsertNode{
			key: "InitProcessEnabled",
			valueToInsert: &yaml.Node{
				Kind:  8,
				Tag:   nodeTagBool,
				Value: "true",
			},
		},
	}
	node1 := &mapUpsertNode{
		upsertNode: upsertNode{
			key:  "Capabilities",
			next: node2,
		},
	}
	return newLinuxParametersNode(node1)
}

func linuxParametersCapabilitiesInitProcessEnabledRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "ContainerDefinitions[0].LinuxParameters.Capabilities.InitProcessEnabled",
		Value: &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   nodeTagBool,
			Value: "true",
		},
	})
}

func newLinuxParametersNode(nextNode nodeUpserter) nodeUpserter {
	node2 := &mapUpsertNode{
		upsertNode: upsertNode{
			key:  "LinuxParameters",
			next: nextNode,
		},
	}
	node1 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:  "ContainerDefinitions",
			next: node2,
		},
		index: 0,
	}
	return newTaskDefPropertyNode(node1)
}

func ulimitsNode() nodeUpserter {
	var node yaml.Node
	_ = yaml.Unmarshal([]byte("ContainerDefinitions[0].Ulimits[-].HardLimit: !Ref ParamName"), &node)

	node3 := &mapUpsertNode{
		upsertNode: upsertNode{
			key:           "HardLimit",
			valueToInsert: node.Content[0].Content[1],
		},
	}
	node2 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:  "Ulimits",
			next: node3,
		},
		appendToLast: true,
	}
	node1 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:  "ContainerDefinitions",
			next: node2,
		},
		index: 0,
	}
	return newTaskDefPropertyNode(node1)
}

func ulimitsRule() Rule {
	return newTaskDefPropertyRule(Rule{
		Path: "ContainerDefinitions[0].Ulimits[-].HardLimit",
		Value: &yaml.Node{
			Kind:  yaml.ScalarNode,
			Style: yaml.TaggedStyle,
			Tag:   "!Ref",
			Value: "ParamName",
		},
	})
}

func exposeExtraPortNode() nodeUpserter {
	node3 := &mapUpsertNode{
		upsertNode: upsertNode{
			key: "ContainerPort",
			valueToInsert: &yaml.Node{
				Kind:  8,
				Tag:   nodeTagInt,
				Value: "5000",
			},
		},
	}
	node2 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:  "PortMappings",
			next: node3,
		},
		appendToLast: true,
	}
	node1 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:  "ContainerDefinitions",
			next: node2,
		},
		index: 0,
	}
	return newTaskDefPropertyNode(node1)
}

func exposeExtraPortRule() Rule {
	return Rule{
		Path: "Resources.TaskDefinition.Properties.ContainerDefinitions[0].PortMappings[-].ContainerPort",
		Value: &yaml.Node{
			Kind:  8,
			Tag:   nodeTagInt,
			Value: "5000",
		},
	}
}

func referBadSeqIndexNode() nodeUpserter {
	var node yaml.Node
	_ = yaml.Unmarshal([]byte("ContainerDefinitions[0].PortMappings[1].ContainerPort: 5000"), &node)

	node3 := &mapUpsertNode{
		upsertNode: upsertNode{
			key:           "ContainerPort",
			valueToInsert: node.Content[0].Content[1],
		},
	}
	node2 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:  "PortMappings",
			next: node3,
		},
		index: 1,
	}
	node1 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:  "ContainerDefinitions",
			next: node2,
		},
		index: 0,
	}
	return newTaskDefPropertyNode(node1)
}

func referBadSeqIndexWithNoKeyNode() nodeUpserter {
	var node yaml.Node
	_ = yaml.Unmarshal([]byte("ContainerDefinitions[0].VolumesFrom[1].SourceContainer: foo"), &node)

	node3 := &mapUpsertNode{
		upsertNode: upsertNode{
			key:           "SourceContainer",
			valueToInsert: node.Content[0].Content[1],
		},
	}
	node2 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:  "VolumesFrom",
			next: node3,
		},
		index: 1,
	}
	node1 := &seqIdxUpsertNode{
		upsertNode: upsertNode{
			key:  "ContainerDefinitions",
			next: node2,
		},
		index: 0,
	}
	return newTaskDefPropertyNode(node1)
}

func Test_applyRules(t *testing.T) {
	testCases := map[string]struct {
		inRules   []nodeUpserter
		inContent string

		wantedContent string
		wantedError   error
	}{
		"error when invalid CFN template": {
			inContent: "",

			wantedError: fmt.Errorf("cannot apply override rule on empty YAML template"),
		},
		"error when referring to bad sequence index": {
			inContent: testContent,
			inRules: []nodeUpserter{
				referBadSeqIndexNode(),
			},

			wantedError: fmt.Errorf("cannot specify PortMappings[1] because the current length is 1. Use [%s] to append to the sequence instead", seqAppendToLastSymbol),
		},
		"error when referring to bad sequence index when sequence key doesn't exist": {
			inContent: testContent,
			inRules: []nodeUpserter{
				referBadSeqIndexWithNoKeyNode(),
			},

			wantedError: fmt.Errorf("cannot specify VolumesFrom[1] because VolumesFrom does not exist. Use VolumesFrom[%s] to append to the sequence instead", seqAppendToLastSymbol),
		},
		"success": {
			inContent: testContent,
			inRules: []nodeUpserter{
				ulimitsNode(),
				exposeExtraPortNode(),
				linuxParametersCapabilitiesNode(),
				linuxParametersCapabilitiesInitProcessEnabledNode(),
				requiresCompatibilitiesNode(),
			},
			wantedContent: wantedOverriddenTemplate,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			var node yaml.Node
			unmarshalErr := yaml.Unmarshal([]byte(tc.inContent), &node)
			require.NoError(t, unmarshalErr)

			// WHEN
			err := applyRules(tc.inRules, &node)
			out, marshalErr := marshalCFNYAML(&node)
			require.NoError(t, marshalErr)

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, strings.TrimSpace(tc.wantedContent), strings.TrimSpace(string(out)))
			}
		})
	}
}

func Test_CloudFormationTemplate(t *testing.T) {
	testCases := map[string]struct {
		inRules    []Rule
		inOrigTemp []byte

		wantedErr error
		wantedOut string
	}{
		"success": {
			inRules: []Rule{
				ulimitsRule(),
				exposeExtraPortRule(),
				linuxParametersCapabilitiesRule(),
				linuxParametersCapabilitiesInitProcessEnabledRule(),
				requiresCompatibilitiesRule(),
			},
			inOrigTemp: []byte(testContent),
			wantedOut:  wantedOverriddenTemplate,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := CloudFormationTemplate(tc.inRules, tc.inOrigTemp)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedOut, string(got))
			}
		})
	}
}
