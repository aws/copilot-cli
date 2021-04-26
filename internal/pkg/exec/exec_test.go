// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/exec/mocks"
	"github.com/golang/mock/gomock"
)

func TestCmd_Run(t *testing.T) {
	t.Run("should delegate to exec and call Run", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cmd := &Cmd{
			command: func(name string, args []string, opts ...CmdOption) cmdRunner {
				require.Equal(t, "ls", name)
				m := mocks.NewMockcmdRunner(ctrl)
				m.EXPECT().Run().Return(nil)
				return m
			},
		}

		// WHEN
		err := cmd.Run("ls", nil)

		// THEN
		require.NoError(t, err)
	})
}
