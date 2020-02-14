// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestTemplate(t *testing.T) {
	const (
		paramsTemplate = `
Project:
	Type: String
	Description: The project name.
Env:
	Type: String
	Description: The environment name your application is being deployed to.
App:
	Type: String
	Description: The name of the application being deployed.`
		policyTemplate = `
AdditionalResourcesPolicy:
  Type: AWS::IAM::ManagedPolicy
  Properties:
    PolicyName: AdditionalResourcesPolicy
    PolicyDocument:
      Version: 2012-10-17
      Statement:
      - Effect: Allow
        Action:
        - "dynamodb:BatchGet*"
				Resource: "arn:aws:dynamodb:*:*:table/{{tableName}}"`
		resourceTemplate = `
KudosDynamoDBTable:
  Type: AWS::DynamoDB::Table
  Properties: 
    TableName: !Sub "${Project}-${Env}-${App}-kudos"
    AttributeDefinitions: 
      - AttributeName: id
        AttributeType: S
    KeySchema: 
      - AttributeName: id
        KeyType: HASH
    ProvisionedThroughput: 
      ReadCapacityUnits: 5
			WriteCapacityUnits: 5`
		outputsTemplate = `
AdditionalResourcesPolicyArn:
  Description: ARN of the policy to access additional resources from the application.
  Value: !Ref AdditionalResourcesPolicy`
	)
	testCases := map[string]struct {
		appName string

		mockWorkspace func(m *mocks.MockworkspaceService)

		wantTemplate string
		wantErr      error
	}{
		"should return addon template": {
			appName: "my-app",

			mockWorkspace: func(m *mocks.MockworkspaceService) {
				m.EXPECT().ListAddonsFiles("my-app").Return([]string{"params.yml", "kudos-DynamoDBTable.yml", "outputs.yml", "policy.yml"}, nil)
				m.EXPECT().ReadAddonsFile("my-app", "params.yml").Return([]byte(paramsTemplate), nil)
				m.EXPECT().ReadAddonsFile("my-app", "kudos-DynamoDBTable.yml").Return([]byte(resourceTemplate), nil)
				m.EXPECT().ReadAddonsFile("my-app", "outputs.yml").Return([]byte(outputsTemplate), nil)
				m.EXPECT().ReadAddonsFile("my-app", "policy.yml").Return([]byte(policyTemplate), nil)
			},

			wantTemplate: `Parameters:  
  Project:
  	Type: String
  	Description: The project name.
  Env:
  	Type: String
  	Description: The environment name your application is being deployed to.
  App:
  	Type: String
  	Description: The name of the application being deployed.
Resources:  
  KudosDynamoDBTable:
    Type: AWS::DynamoDB::Table
    Properties: 
      TableName: !Sub "${Project}-${Env}-${App}-kudos"
      AttributeDefinitions: 
        - AttributeName: id
          AttributeType: S
      KeySchema: 
        - AttributeName: id
          KeyType: HASH
      ProvisionedThroughput: 
        ReadCapacityUnits: 5
  			WriteCapacityUnits: 5
  
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyName: AdditionalResourcesPolicy
      PolicyDocument:
        Version: 2012-10-17
        Statement:
        - Effect: Allow
          Action:
          - "dynamodb:BatchGet*"
  				Resource: "arn:aws:dynamodb:*:*:table/{{tableName}}"
Outputs:  
  AdditionalResourcesPolicyArn:
    Description: ARN of the policy to access additional resources from the application.
    Value: !Ref AdditionalResourcesPolicy
`,
			wantErr: nil,
		},
		"should return empty string if no addon files found": {
			appName: "my-app",

			mockWorkspace: func(m *mocks.MockworkspaceService) {
				m.EXPECT().ListAddonsFiles("my-app").Return([]string{}, nil)
			},

			wantTemplate: "",
			wantErr:      nil,
		},
		"should return error if fail to list addon files": {
			appName: "my-app",

			mockWorkspace: func(m *mocks.MockworkspaceService) {
				m.EXPECT().ListAddonsFiles("my-app").Return(nil, errors.New("some error"))
			},

			wantErr: fmt.Errorf("list addon files: %w", errors.New("some error")),
		},
		"should return error if fail to read files": {
			appName: "my-app",

			mockWorkspace: func(m *mocks.MockworkspaceService) {
				m.EXPECT().ListAddonsFiles("my-app").Return([]string{"params.yml", "kudos-DynamoDBTable.yml", "outputs.yml", "policy.yml"}, nil)
				m.EXPECT().ReadAddonsFile("my-app", "params.yml").Return(nil, errors.New("some error"))
			},

			wantErr: fmt.Errorf("read addon file %s: %w", "params.yml", errors.New("some error")),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace := mocks.NewMockworkspaceService(ctrl)
			tc.mockWorkspace(mockWorkspace)

			service := Addons{
				appName: tc.appName,
				ws:      mockWorkspace,
			}

			gotTemplate, gotErr := service.Template()

			if gotErr != nil {
				require.Equal(t, tc.wantErr, gotErr)
			} else {
				require.Equal(t, tc.wantTemplate, gotTemplate)
			}
		})
	}
}
