// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"context"
	"os"
	"os/signal"
)

// InteractiveRun runs the input command that starts a child process.
func (c *Cmd) InteractiveRun(name string, args []string) error {
	sig := make(chan os.Signal, 1)
	// See https://golang.org/pkg/os/signal/#hdr-Windows
	signal.Notify(sig, os.Interrupt)
	defer signal.Reset(os.Interrupt)
	cmd := c.command(context.Background(), name, args, Stdout(os.Stdout), Stdin(os.Stdin), Stderr(os.Stderr))
	return cmd.Run()
}
