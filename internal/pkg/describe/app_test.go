// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAppDescriber_Version(t *testing.T) {
	testCases := map[string]struct {
		given func(ctrl *gomock.Controller) *AppDescriber

		wantedVersion string
		wantedErr     error
	}{
		"should return error if fail to get metadata": {
			given: func(ctrl *gomock.Controller) *AppDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().StackMetadata().Return("", errors.New("some error"))
				return &AppDescriber{
					app:               "phonetool",
					stackDescriber:    m,
					stackSetDescriber: m,
				}
			},
			wantedErr: fmt.Errorf("some error"),
		},
		"success": {
			given: func(ctrl *gomock.Controller) *AppDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().StackMetadata().Return(`{"TemplateVersion":"v1.2.0"}`, nil)
				m.EXPECT().StackSetMetadata().Return(`{"TemplateVersion":"v1.0.0"}`, nil)
				return &AppDescriber{
					app:               "phonetool",
					stackDescriber:    m,
					stackSetDescriber: m,
				}
			},

			wantedVersion: "v1.0.0",
		},
		"success with legacy template": {
			given: func(ctrl *gomock.Controller) *AppDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().StackMetadata().Return("", nil)
				m.EXPECT().StackSetMetadata().Return(`{"TemplateVersion":"v1.0.0"}`, nil)
				return &AppDescriber{
					app:               "phonetool",
					stackDescriber:    m,
					stackSetDescriber: m,
				}
			},

			wantedVersion: "v0.0.0",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			d := tc.given(ctrl)

			// WHEN
			actual, err := d.Version()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedVersion, actual)
			}
		})
	}
}
