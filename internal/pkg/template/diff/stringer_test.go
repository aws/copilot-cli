// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"testing"
)

func Test_Integration_Parse_Write(t *testing.T) {
	testCases := map[string]struct {
		curr        string
		old         string
		wanted      string
		wantedError error
	}{
		"add a map": {
			curr: `Mary:
  Height:
    cm: 168
  Weight:
    kg: 52`,
			old: `Mary:
  Height:
    cm: 168`,

			wanted: `~ Mary:
    + Weight:
    +     kg: 52
`,
		},
		"remove a map": {
			curr: `Mary:
  Height:
    cm: 168`,
			old: `Mary:
  Height:
    cm: 168
  Weight:
    kg: 52`,
			wanted: `~ Mary:
    - Weight:
    -     kg: 52
`,
		},
		"change keyed values": {
			curr: `Mary:
  Height:
    cm: 168`,
			old: `Mary:
  Height:
    cm: 190`,
			wanted: `~ Mary:
    ~ Height:
        ~ cm: 190 -> 168
`,
		},
		"list does not change": {
			old:    `Alphabet: [a,b,c,d]`,
			curr:   `Alphabet: [a,b,c,d]`,
			wanted: "No changes.\n",
		},
		"list reordered": {
			// TODO(lou1425926): complete the test.
			old:  `SizeRank: [bear,dog,cat,mouse]`,
			curr: `SizeRank: [bear,cat,dog,mouse]`,
		},
		"list with insertion": {
			old:  `DanceCompetition: [dog,bear,cat]`,
			curr: `DanceCompetition: [dog,bear,mouse,cat]`,
			wanted: `~ DanceCompetition:
    + - mouse
`,
		},
		"list with deletion": {
			old:  `PotatoChipCommittee: [dog,bear,cat,mouse]`,
			curr: `PotatoChipCommittee: [dog,bear,mouse]`,
			wanted: `~ PotatoChipCommittee:
    - - cat
`,
		},
		"list with a scalar value changed": {
			old:  `DogsFavoriteShape: [triangle,circle,rectangle]`,
			curr: `DogsFavoriteShape: [triangle,ellipse,rectangle]`,
			wanted: `~ DogsFavoriteShape:
    ~ - circle -> ellipse
`,
		},
		"list with a map value changed": { // TODO(lou1415926): handle list of maps modification
			old: `StrawberryPopularitySurvey:
- Name: Dog
  LikeStrawberry: ver much
- Name: Bear
  LikeStrawberry: meh
  D: 
     - One
     - Three:
          Wow: what
- Name: Cat
  LikeStrawberry: ew`,
			curr: `StrawberryPopularitySurvey:
- Name: Dog
  LikeStrawberry: ver much
- Name: Bear
  LikeStrawberry: ok
  Hey: wow
  D:
     - Two
     - Three: 
         Wow: hey
- Name: Cat
  LikeStrawberry: ew`,
		},
		"change a map to scalar": {
			curr: `Mary:
  Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you"`,
			old: `Mary:
  Dialogue:
    Bear: "I know I'm supposed to keep an eye on you"`,
			wanted: `~ Mary:
    - Dialogue:
    -     Bear: "I know I'm supposed to keep an eye on you"
    + Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you"
`,
		},
		"change a list to scalar": {
			curr: `Mary:
  Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you; Said Dog: 'ikr'"`,
			old: `Mary:
  Dialogue:
    - Bear: "I know I'm supposed to keep an eye on you"
      Tone: disappointed
    - Dog: "ikr"
      Tone: pleased`,
			wanted: `~ Mary:
    - Dialogue:
    -     - Bear: "I know I'm supposed to keep an eye on you"
    -       Tone: disappointed
    -     - Dog: "ikr"
    -       Tone: pleased
    + Dialogue: "Said bear: 'I know I'm supposed to keep an eye on you; Said Dog: 'ikr'"
`,
		},
		"change a map to list": {
			curr: `Mary:
  Dialogue:
    - Bear: "I know I'm supposed to keep an eye on you"
      Tone: disappointed
    - Dog: "ikr"
      Tone: pleased`,
			old: `Mary:
  Dialogue:
    Bear: (disappointed) "I know I'm supposed to keep an eye on you"
    Dog: (pleased) "ikr"`,
			wanted: `~ Mary:
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
			wanted: `~ DogsFavoriteShape:
    - - irregular
    ~ - circle -> ellipse
    + - food-shape
`,
		},
		"no diff": {
			curr: `Mary:
  Height:
    cm: 190
  CanFight: yes
  FavoriteWord: muscle`,

			old: `Mary:
  Height:
    cm: 190
  CanFight: yes
  FavoriteWord: muscle`,
			wanted: "No changes.\n",
		},
	}
	for name, _ := range testCases {
		t.Run(name, func(t *testing.T) {
		})
	}
}
