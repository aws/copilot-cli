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

func newTaskDefPropertyNode(nextNode *ruleNode) *ruleNode {
	end := &ruleNode{
		name:      "Properties",
		valueType: mapType,
		next:      nextNode,
	}
	taskDefNode := &ruleNode{
		name:      "TaskDefinition",
		valueType: mapType,
		next:      end,
	}
	head := &ruleNode{
		name:      "Resources",
		valueType: mapType,
		next:      taskDefNode,
	}
	return head
}

func addLinuxParametersCapabilities() *ruleNode {
	var node yaml.Node
	_ = yaml.Unmarshal([]byte(`ContainerDefinitions[0].LinuxParameters.Capabilities.Add: ["AUDIT_CONTROL", "AUDIT_WRITE"]`), &node)

	node2 := &ruleNode{
		name:         "Add",
		valueType:    endNodeType,
		endNodeValue: node.Content[0].Content[1],
	}
	node1 := &ruleNode{
		name:      "Capabilities",
		valueType: mapType,
		next:      node2,
	}
	return newLinuxParameters(node1)
}

func addLinuxParametersCapabilitiesInitProcessEnabled() *ruleNode {
	node2 := &ruleNode{
		name:      "InitProcessEnabled",
		valueType: endNodeType,
		endNodeValue: &yaml.Node{
			Kind:  8,
			Tag:   nodeTagBool,
			Value: "true",
		},
	}
	node1 := &ruleNode{
		name:      "Capabilities",
		valueType: mapType,
		next:      node2,
	}
	return newLinuxParameters(node1)
}

func newLinuxParameters(nextNode *ruleNode) *ruleNode {
	node2 := &ruleNode{
		name:      "LinuxParameters",
		valueType: mapType,
		next:      nextNode,
	}
	node1 := &ruleNode{
		name:      "ContainerDefinitions",
		valueType: seqType,
		seqValue:  nodeSeqValue{index: 0},
		next:      node2,
	}
	return newTaskDefPropertyNode(node1)
}

func addUlimits() *ruleNode {
	var node yaml.Node
	_ = yaml.Unmarshal([]byte("ContainerDefinitions[0].Ulimit[0].Hardlimit: !Ref ParamName"), &node)

	node3 := &ruleNode{
		name:         "HardLimit",
		valueType:    endNodeType,
		endNodeValue: node.Content[0].Content[1],
	}
	node2 := &ruleNode{
		name:      "Ulimits",
		valueType: seqType,
		seqValue: nodeSeqValue{
			appendToLast: true,
		},
		next: node3,
	}
	node1 := &ruleNode{
		name:      "ContainerDefinitions",
		valueType: seqType,
		seqValue:  nodeSeqValue{index: 0},
		next:      node2,
	}
	return newTaskDefPropertyNode(node1)
}

func exposeExtraPort() *ruleNode {
	node3 := &ruleNode{
		name:      "ContainerPort",
		valueType: endNodeType,
		endNodeValue: &yaml.Node{
			Kind:  8,
			Tag:   nodeTagInt,
			Value: "5000",
		},
	}
	node2 := &ruleNode{
		name:      "PortMappings",
		valueType: seqType,
		seqValue: nodeSeqValue{
			appendToLast: true,
		},
		next: node3,
	}
	node1 := &ruleNode{
		name:      "ContainerDefinitions",
		valueType: seqType,
		seqValue:  nodeSeqValue{index: 0},
		next:      node2,
	}
	return newTaskDefPropertyNode(node1)
}

func referBadSeqIndex() *ruleNode {
	var node yaml.Node
	_ = yaml.Unmarshal([]byte("ContainerDefinitions[0].PortMappings[2].ContainerPort: 5000"), &node)

	node3 := &ruleNode{
		name:         "ContainerPort",
		valueType:    endNodeType,
		endNodeValue: node.Content[0].Content[1],
	}
	node2 := &ruleNode{
		name:      "PortMappings",
		valueType: seqType,
		seqValue: nodeSeqValue{
			index: 1,
		},
		next: node3,
	}
	node1 := &ruleNode{
		name:      "ContainerDefinitions",
		valueType: seqType,
		seqValue:  nodeSeqValue{index: 0},
		next:      node2,
	}
	return newTaskDefPropertyNode(node1)
}

func Test_applyRulesToCFNTemplate(t *testing.T) {
	testCases := map[string]struct {
		inRules   []*ruleNode
		inContent string

		wantedContent string
		wantedError   error
	}{
		"error when invalid CFN template": {
			inContent: "",

			wantedError: fmt.Errorf("cannot apply override rule on empty CloudFormation template"),
		},
		"error when referring to bad sequence index": {
			inContent: testContent,
			inRules: []*ruleNode{
				referBadSeqIndex(),
			},

			wantedError: fmt.Errorf("cannot specify PortMappings[1] because the current length is 1. Use [+] to append to the sequence instead"),
		},
		"success": {
			inContent: testContent,
			inRules: []*ruleNode{
				addUlimits(),
				exposeExtraPort(),
				addLinuxParametersCapabilities(),
				addLinuxParametersCapabilitiesInitProcessEnabled(),
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
