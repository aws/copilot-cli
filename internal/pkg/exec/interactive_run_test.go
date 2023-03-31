// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestCmd_InteractiveRun(t *testing.T) {
	t.Run("should enable default stdin, stdout, and stderr before running the command", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cmd := &Cmd{
			command: func(ctx context.Context, name string, args []string, opts ...CmdOption) cmdRunner {
				// Ensure that the options applied match what we expect.
				cmd := &exec.Cmd{}
				for _, opt := range opts {
					opt(cmd)
				}
				require.Equal(t, os.Stdin, cmd.Stdin)
				require.Equal(t, os.Stdout, cmd.Stdout)
				require.Equal(t, os.Stderr, cmd.Stderr)

				m := NewMockcmdRunner(ctrl)
				m.EXPECT().Run().Return(nil)
				return m
			},
		}

		// WHEN
		err := cmd.InteractiveRun("hello", nil)

		// THEN
		require.NoError(t, err)
	})
}
