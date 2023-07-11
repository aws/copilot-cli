// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package signal

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignal(t *testing.T) {
	testCases := map[string]struct {
		inSignals []os.Signal
	}{
		"no signal received": {},
		"recieve a single signal": {
			inSignals: []os.Signal{syscall.SIGINT},
		},

		"recieve a couple of signals": {
			inSignals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			sig := &Signal{
				signalCh: make(chan os.Signal),
				sigs:     tc.inSignals,
			}
			// WHEN
			signalCh := sig.NotifySignals()

			// THEN
			for _, expectedSignal := range tc.inSignals {
				if err := syscall.Kill(syscall.Getpid(), expectedSignal.(syscall.Signal)); err != nil {
					require.Error(t, err)
				}
				if sig := <-signalCh; sig != expectedSignal {
					t.Errorf("wanted signal is %v, Got signal %v", expectedSignal, sig)
				}
			}
			sig.StopCatchSignals()
		})
	}
}
