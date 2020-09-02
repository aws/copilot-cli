// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudwatchlogs

import (
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
	c "github.com/fatih/color"
	"github.com/stretchr/testify/require"
)

func TestColorCodeMessage(t *testing.T) {
	color.DisableColorBasedOnEnvVar()
	testCases := map[string]struct {
		givenMessage string
		givenCode    string
		givenColor   *c.Color
		wantMessage  string
	}{
		"should not apply color to a fatal code if it exists in a message as a substring": {
			givenMessage: "e2e environment variables have been OVERRIDEN",
			givenCode:    "ERR",
			givenColor:   color.Red,
			wantMessage:  "e2e environment variables have been OVERRIDEN",
		},
		"should apply color to a fatal code if exists in a message": {
			givenMessage: "An Error has occured",
			givenCode:    "Error",
			givenColor:   color.Red,
			wantMessage:  fmt.Sprintf("An %s has occured", color.Red.Sprint("Error")),
		},
		"should not apply color to a warning code if exists in a message as a substring": {
			givenMessage: "Forewarning",
			givenCode:    "warning",
			givenColor:   color.Yellow,
			wantMessage:  "Forewarning",
		},
		"should apply color to a warning code if exists in a message": {
			givenMessage: "Warning something has happened",
			givenCode:    "Warning",
			givenColor:   color.Yellow,
			wantMessage:  fmt.Sprintf("%s something has happened", color.Yellow.Sprint("Warning")),
		},
		"should apply color to a fatal code if code is next to special character": {
			givenMessage: "error: something happened",
			givenCode:    "error",
			givenColor:   color.Red,
			wantMessage:  fmt.Sprintf("%s: something happened", color.Red.Sprint("error")),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			color.DisableColorBasedOnEnvVar()
			got := colorCodeMessage(tc.givenMessage, tc.givenCode, tc.givenColor)
			require.Equal(t, tc.wantMessage, got)
		})
	}
}
