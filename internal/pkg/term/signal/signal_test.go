// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package signal

import (
	"os"
	"syscall"
	"testing"
)

func TestSignal(t *testing.T) {
	sig := &Signal{
		signalCh: make(chan os.Signal),
		sigs:     []os.Signal{syscall.SIGINT},
	}
	signalCh := sig.NotifySignals()
	err := syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	if err == nil {
		if sig := <-signalCh; sig != syscall.SIGINT {
			t.Errorf("wanted signal is %v,Got signal %v", syscall.SIGINT, sig)
		}
	}
	sig.StopCatchSignals()
}
