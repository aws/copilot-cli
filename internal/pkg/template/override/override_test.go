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

const testContent = `
Resources:
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

const overridenTestContent = `
Resources:
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

func newTaskDefPropertyNode(nextNode contentUpserter) contentUpserter {
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

func addRequiresCompatibilities() contentUpserter {
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

func addLinuxParametersCapabilities() contentUpserter {
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
	return newLinuxParameters(node1)
}

func addLinuxParametersCapabilitiesInitProcessEnabled() contentUpserter {
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
	return newLinuxParameters(node1)
}

func newLinuxParameters(nextNode contentUpserter) contentUpserter {
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

func addUlimits() contentUpserter {
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

func exposeExtraPort() contentUpserter {
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

func referBadSeqIndex() contentUpserter {
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

func Test_applyRulesToCFNTemplate(t *testing.T) {
	testCases := map[string]struct {
		inRules   []contentUpserter
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
			inRules: []contentUpserter{
				referBadSeqIndex(),
			},

			wantedError: fmt.Errorf("cannot specify PortMappings[1] because the current length is 1. Use [%s] to append to the sequence instead", seqAppendToLastSymbol),
		},
		"success": {
			inContent: testContent,
			inRules: []contentUpserter{
				addUlimits(),
				exposeExtraPort(),
				addLinuxParametersCapabilities(),
				addLinuxParametersCapabilitiesInitProcessEnabled(),
				addRequiresCompatibilities(),
			},
			wantedContent: overridenTestContent,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			var node yaml.Node
			unmarshalErr := yaml.Unmarshal([]byte(tc.inContent), &node)
			require.NoError(t, unmarshalErr)

			// WHEN
			err := applyRulesToCFNTemplate(tc.inRules, &node)
			out, marshalErr := yaml.Marshal(&node)
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
