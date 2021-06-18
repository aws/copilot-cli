// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package summarybar

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_New(t *testing.T) {
	testCases := map[string]struct {
		inData     []Datum
		inWidth    int
		inEmptyRep string
		inOpts     []Opt

		wantedSummaryBarComponent *summaryBarComponent
		wantedError               error
	}{
		"returns wanted bar component with no opts": {
			inData: []Datum{
				{
					Value:          1,
					Representation: "W",
				},
				{
					Value:          2,
					Representation: "H",
				},
				{
					Value:          3,
					Representation: "A",
				},
			},
			wantedSummaryBarComponent: &summaryBarComponent{
				width: 0,
				data: []Datum{
					{
						Value:          1,
						Representation: "W",
					},
					{
						Value:          2,
						Representation: "H",
					},
					{
						Value:          3,
						Representation: "A",
					},
				},
				emptyRep: "",
			},
		},
		"returns wanted bar component with width": {
			inData: []Datum{
				{
					Value:          1,
					Representation: "W",
				},
			},
			inOpts: []Opt{WithWidth(20)},
			wantedSummaryBarComponent: &summaryBarComponent{
				width: 20,
				data: []Datum{
					{
						Value:          1,
						Representation: "W",
					},
				},
				emptyRep: "",
			},
		},
		"returns wanted bar component with empty representation": {
			inData: []Datum{
				{
					Value:          1,
					Representation: "W",
				},
			},
			inOpts: []Opt{
				WithEmptyRep("@"),
			},
			wantedSummaryBarComponent: &summaryBarComponent{
				width: 0,
				data: []Datum{
					{
						Value:          1,
						Representation: "W",
					},
				},
				emptyRep: "@",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			component := New(tc.inData, tc.inOpts...)
			require.Equal(t, component, tc.wantedSummaryBarComponent)
		})
	}
}

func TestSummaryBarComponent_Render(t *testing.T) {
	testCases := map[string]struct {
		inData     []Datum
		inWidth    int
		inEmptyRep string

		wantedError error
		wantedOut   string
	}{
		"error if width <= 0": {
			inData:      []Datum{},
			wantedError: errors.New("invalid width 0 for summary bar"),
		},
		"error if data contains negative values": {
			inWidth: 10,
			inData: []Datum{
				{
					Value:          1,
					Representation: "W",
				},
				{
					Value:          0,
					Representation: "H",
				},
				{
					Value:          -1,
					Representation: "A",
				},
			},
			wantedError: errors.New("input data contains negative values"),
		},
		"output empty bar if data is empty": {
			inWidth:    10,
			inEmptyRep: "@",
			inData:     []Datum{},
			wantedOut:  "@@@@@@@@@@",
		},
		"output empty bar if data sum up to 0": {
			inWidth:    10,
			inEmptyRep: "@",
			inData: []Datum{
				{
					Value:          0,
					Representation: "W",
				},
				{
					Value:          0,
					Representation: "H",
				},
				{
					Value:          0,
					Representation: "A",
				},
			},
			wantedOut: "@@@@@@@@@@",
		},
		"output correct bar when data sums up to length": {
			inWidth:    10,
			inEmptyRep: "@",
			inData: []Datum{
				{
					Value:          1,
					Representation: "W",
				},
				{
					Value:          5,
					Representation: "H",
				},
				{
					Value:          4,
					Representation: "A",
				},
			},
			wantedOut: "WHHHHHAAAA",
		},
		"output correct bar when data doesn't sum up to length": {
			inWidth:    10,
			inEmptyRep: "@",
			inData: []Datum{
				{
					Value:          4,
					Representation: "W",
				},
				{
					Value:          2,
					Representation: "H",
				},
				{
					Value:          2,
					Representation: "A",
				},
				{
					Value:          1,
					Representation: "T",
				},
			},
			wantedOut: "WWWWWHHAAT",
		},
		"output correct bar when data sum exceeds length": {
			inWidth:    10,
			inEmptyRep: "@",
			inData: []Datum{
				{
					Value:          4,
					Representation: "W",
				},
				{
					Value:          3,
					Representation: "H",
				},
				{
					Value:          6,
					Representation: "A",
				},
				{
					Value:          3,
					Representation: "T",
				},
			},
			wantedOut: "WWHHAAAATT",
		},
		"output correct bar when data is roughly uniform": {
			inWidth:    10,
			inEmptyRep: "@",
			inData: []Datum{
				{
					Value:          2,
					Representation: "W",
				},
				{
					Value:          3,
					Representation: "H",
				},
				{
					Value:          3,
					Representation: "A",
				},
				{
					Value:          3,
					Representation: "T",
				},
			},
			wantedOut: "WWHHHAAATT",
		},
		"output correct bar when data is heavily skewed": {
			inWidth:    10,
			inEmptyRep: "@",
			inData: []Datum{
				{
					Value:          23,
					Representation: "W",
				},
				{
					Value:          3,
					Representation: "H",
				},
				{
					Value:          3,
					Representation: "A",
				},
				{
					Value:          3,
					Representation: "T",
				},
			},
			wantedOut: "WWWWWWWHAT",
		},
		"output correct bar when data is extremely heavily skewed": {
			inWidth:    10,
			inEmptyRep: "@",
			inData: []Datum{
				{
					Value:          233,
					Representation: "W",
				},
				{
					Value:          3,
					Representation: "H",
				},
				{
					Value:          3,
					Representation: "A",
				},
				{
					Value:          3,
					Representation: "T",
				},
			},
			wantedOut: "WWWWWWWWWW",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			bar := summaryBarComponent{
				data:     tc.inData,
				width:    tc.inWidth,
				emptyRep: tc.inEmptyRep,
			}
			buf := new(strings.Builder)
			_, err := bar.Render(buf)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Equal(t, buf.String(), tc.wantedOut)
			}
		})
	}
}
