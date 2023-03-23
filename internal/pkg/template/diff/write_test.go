// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Integration_Parse_Write(t *testing.T) {
	testCases := map[string]struct {
		curr   string
		old    string
		wanted string
	}{
		"add a map": {
			curr: `
Mary:
  Height:
    cm: 168
  Weight:
    kg: 52`,
			old: `
Mary:
  Height:
    cm: 168`,

			wanted: `
~ Mary:
    + Weight:
    +     kg: 52
`,
		},
		"remove a map": {
			curr: `
Mary:
  Height:
    cm: 168`,
			old: `
Mary:
  Height:
    cm: 168
  Weight:
    kg: 52`,
			wanted: `
~ Mary:
    - Weight:
    -     kg: 52
`,
		},
		"change keyed values": {
			curr: `
Mary:
  Height:
    cm: 168`,
			old: `
Mary:
  Height:
    cm: 190`,
			wanted: `
~ Mary:
    ~ Height:
        ~ cm: 190 -> 168
`,
		},
		"list does not change": {
			old:  `Alphabet: [a,b,c,d]`,
			curr: `Alphabet: [a,b,c,d]`,
		},
		"list with insertion": {
			old:  `DanceCompetition: [dog,bear,cat]`,
			curr: `DanceCompetition: [dog,bear,mouse,cat]`,
			wanted: `
~ DanceCompetition:
    (2 unchanged items)
    + - mouse
    (1 unchanged item)
`,
		},
		"list with deletion": {
			old:  `PotatoChipCommittee: [dog,bear,cat,mouse]`,
			curr: `PotatoChipCommittee: [dog,bear,mouse]`,
			wanted: `
~ PotatoChipCommittee:
    (2 unchanged items)
    - - cat
    (1 unchanged item)
`,
		},
		"list with a scalar value changed": {
			old:  `DogsFavoriteShape: [triangle,circle,rectangle]`,
			curr: `DogsFavoriteShape: [triangle,ellipse,rectangle]`,
			wanted: `
~ DogsFavoriteShape:
    (1 unchanged item)
    ~ - circle -> ellipse
    (1 unchanged item)
`,
		},
		"change a map to scalar": {
			curr: `
Mary:
  Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you"`,
			old: `
Mary:
  Dialogue:
    Bear: "I know I'm supposed to keep an eye on you"`,
			wanted: `
~ Mary:
    - Dialogue:
    -     Bear: "I know I'm supposed to keep an eye on you"
    + Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you"
`,
		},
		"change a list to scalar": {
			curr: `
Mary:
  Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you; Said Dog: 'ikr'"`,
			old: `
Mary:
  Dialogue:
    - Bear: "I know I'm supposed to keep an eye on you"
      Tone: disappointed
    - Dog: "ikr"
      Tone: pleased`,
			wanted: `
~ Mary:
    - Dialogue:
    -     - Bear: "I know I'm supposed to keep an eye on you"
    -       Tone: disappointed
    -     - Dog: "ikr"
    -       Tone: pleased
    + Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you; Said Dog: 'ikr'"
`,
		},
		"change a map to list": {
			curr: `
Mary:
  Dialogue:
    - Bear: "I know I'm supposed to keep an eye on you"
      Tone: disappointed
    - Dog: "ikr"
      Tone: pleased`,
			old: `
Mary:
  Dialogue:
    Bear: (disappointed) "I know I'm supposed to keep an eye on you"
    Dog: (pleased) "ikr"`,
			wanted: `
~ Mary:
    - Dialogue:
    -     Bear: (disappointed) "I know I'm supposed to keep an eye on you"
    -     Dog: (pleased) "ikr"
    + Dialogue:
    +     - Bear: "I know I'm supposed to keep an eye on you"
    +       Tone: disappointed
    +     - Dog: "ikr"
    +       Tone: pleased
`,
		},
		"list with scalar insertion, deletion and value changed": {
			old:  `DogsFavoriteShape: [irregular,triangle,circle,rectangle]`,
			curr: `DogsFavoriteShape: [triangle,ellipse,rectangle,food-shape]`,
			wanted: `
~ DogsFavoriteShape:
    - - irregular
    (1 unchanged item)
    ~ - circle -> ellipse
    (1 unchanged item)
    + - food-shape
`,
		},
		"from is empty": {
			curr: `Mary: likes animals`,
			old:  `   `,
			wanted: `+ Mary: likes animals
`,
		},
		"to is empty": {
			curr: `           `,
			old:  `Mary: likes animals`,
			wanted: `- Mary: likes animals
`,
		},
		"from and to are both empty": {
			curr: `           `,
			old:  `  `,
		},
		"no diff": {
			curr: `
Mary:
  Height:
    cm: 190
  CanFight: yes
  FavoriteWord: muscle`,

			old: `
Mary:
  Height:
    cm: 190
  CanFight: yes
  FavoriteWord: muscle`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotTree, err := From(tc.old).Parse([]byte(tc.curr))
			require.NoError(t, err)

			buf := strings.Builder{}
			err = gotTree.Write(&buf)
			out := buf.String()
			require.NoError(t, err)
			require.Equal(t, strings.TrimPrefix(tc.wanted, "\n"), out)
		})
	}
}

func Test_Integration_Parse_Write_WithCFNIgnorer(t *testing.T) {
	testCases := map[string]struct {
		curr      string
		old       string
		overrider overrider
		wanted    string
	}{
		"ignore metadata manifest": {
			old: `Description: CloudFormation environment template for infrastructure shared among Copilot workloads.
Metadata:
  Version: v1.26.0
  Manifest: I don't see any difference.`,
			curr: `Description: CloudFormation environment template for infrastructure shared among Copilot workloads.
Metadata:
  Version: v1.27.0
  Manifest: There is definitely a difference.`,
			wanted: `
~ Metadata:
    ~ Version: v1.26.0 -> v1.27.0
`,
		},
		"no diff": {
			old: `Description: CloudFormation environment template for infrastructure shared among Copilot workloads.
Metadata:
  Manifest: I don't see any difference.`,
			curr: `Description: CloudFormation environment template for infrastructure shared among Copilot workloads.
Metadata:
  Manifest: There is definitely a difference.`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotTree, err := From(tc.old).ParseWithCFNIgnorer([]byte(tc.curr))
			require.NoError(t, err)
			buf := strings.Builder{}
			err = gotTree.Write(&buf)
			require.NoError(t, err)
			require.Equal(t, strings.TrimPrefix(tc.wanted, "\n"), buf.String())
		})
	}
}
