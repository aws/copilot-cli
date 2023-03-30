// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/golang/mock/gomock"
)

func TestCmd_Run(t *testing.T) {
	t.Run("should delegate to exec and call Run", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cmd := &Cmd{
			command: func(ctx context.Context, name string, args []string, opts ...CmdOption) cmdRunner {
				require.Equal(t, "ls", name)
				m := NewMockcmdRunner(ctrl)
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

func TestCmd_RunWithContext(t *testing.T) {
	t.Run("should delegate to exec and call Run", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cmd := &Cmd{
			command: func(ctx context.Context, name string, args []string, opts ...CmdOption) cmdRunner {
				require.Equal(t, "ls", name)
				m := NewMockcmdRunner(ctrl)
				m.EXPECT().Run().Return(nil)
				return m
			},
		}

		// WHEN
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		err := cmd.RunWithContext(ctx, "ls", nil)

		// THEN
		require.NoError(t, err)
	})
}
