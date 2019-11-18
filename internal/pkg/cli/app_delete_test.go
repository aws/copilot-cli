// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestConfirmDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mocks.NewMockprompter(ctrl)

	mockAppName := "mockAppName"
	mockProjectName := "mockProjectName"
	mockError := errors.New("mockError")

	tests := map[string]struct {
		skipConfirmation bool

		setupMocks func()

		want error
	}{
		"should skip confirmation": {
			skipConfirmation: true,
			setupMocks:       func() {},
			want:             nil,
		},
		"should wrap error returned from prompter confirmation": {
			skipConfirmation: false,
			setupMocks: func() {
				mockPrompter.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, mockAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(true, mockError)
			},
			want: fmt.Errorf("app delete confirmation prompt: %w", mockError),
		},
		"should return error if user does not confirm app deletion": {
			skipConfirmation: false,
			setupMocks: func() {
				mockPrompter.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, mockAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(false, nil)
			},
			want: errAppDeleteCancelled,
		},
		"should return error nil if user confirms app delete": {
			skipConfirmation: false,
			setupMocks: func() {
				mockPrompter.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, mockAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(true, nil)
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.setupMocks()
			opts := deleteAppOpts{
				GlobalOpts: &GlobalOpts{
					projectName: mockProjectName,
				},
				app:              mockAppName,
				skipConfirmation: test.skipConfirmation,
				prompter:         mockPrompter,
			}

			got := opts.confirmDelete()

			require.Equal(t, test.want, got)
		})
	}
}
