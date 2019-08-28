// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/require"
)

func TestStore_List(t *testing.T) {
	testCases := map[string]struct {
		mockGetParametersByPath func(t *testing.T, param *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)

		wantedProjectNames []string
		wantedErr          error
	}{
		"with multiple existing projects": {
			mockGetParametersByPath: func(t *testing.T, param *ssm.GetParametersByPathInput) (output *ssm.GetParametersByPathOutput, e error) {
				require.Equal(t, projectsParamPath, *param.Path)
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							Name: aws.String("/archer/chicken"),
						},
						{
							Name: aws.String("/archer/cow"),
						},
					},
				}, nil
			},

			wantedProjectNames: []string{"chicken", "cow"},
			wantedErr:          nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			store := &Store{
				c: &mockSSM{
					t:                       t,
					mockGetParametersByPath: tc.mockGetParametersByPath,
				},
			}

			// WHEN
			projects, _ := store.List()

			// THEN
			var names []string
			for _, p := range projects {
				names = append(names, p.Name)
			}
			require.Equal(t, tc.wantedProjectNames, names)
		})
	}
}
