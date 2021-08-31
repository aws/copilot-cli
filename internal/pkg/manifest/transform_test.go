// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/imdario/mergo"
	"github.com/stretchr/testify/require"
)

func TestBasicTransformer_Transformer(t *testing.T) {
	type testBasicTransformerStruct struct {
		PBool   *bool
		PInt    *int
		PInt64  *int64
		PUint16 *uint16
		PUint32 *uint32
		PString *string
		PSlice  *[]string
		Slice   []string
	}

	testCases := map[string]struct {
		original func(s *testBasicTransformerStruct)
		override func(s *testBasicTransformerStruct)
		wanted   func(s *testBasicTransformerStruct)
	}{
		"PBool overridden": {
			original: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(false)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
			},
		},
		"PBool explicitly overridden by zero value": {
			original: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(false)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(false)
			},
		},
		"PBool not overridden by nil": {
			original: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
			},
			override: func(s *testBasicTransformerStruct) {},
			wanted: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
			},
		},
		"PInt overridden": {
			original: func(s *testBasicTransformerStruct) {
				s.PInt = aws.Int(24)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PInt = aws.Int(42)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PInt = aws.Int(42)
			},
		},
		"PInt explicitly overridden by zero value": {
			original: func(s *testBasicTransformerStruct) {
				s.PInt = aws.Int(24)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PInt = aws.Int(0)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PInt = aws.Int(0)
			},
		},
		"PInt not overridden by nil": {
			original: func(s *testBasicTransformerStruct) {
				s.PInt = aws.Int(24)
			},
			override: func(s *testBasicTransformerStruct) {},
			wanted: func(s *testBasicTransformerStruct) {
				s.PInt = aws.Int(24)
			},
		},
		"PInt64 overridden": {
			original: func(s *testBasicTransformerStruct) {
				s.PInt64 = aws.Int64(24)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PInt64 = aws.Int64(42)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PInt64 = aws.Int64(42)
			},
		},
		"PInt64 explicitly overridden by zero value": {
			original: func(s *testBasicTransformerStruct) {
				s.PInt64 = aws.Int64(24)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PInt64 = aws.Int64(0)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PInt64 = aws.Int64(0)
			},
		},
		"PInt64 not overridden by nil": {
			original: func(s *testBasicTransformerStruct) {
				s.PInt64 = aws.Int64(24)
			},
			override: func(s *testBasicTransformerStruct) {},
			wanted: func(s *testBasicTransformerStruct) {
				s.PInt64 = aws.Int64(24)
			},
		},
		"PUint16 overridden": {
			original: func(s *testBasicTransformerStruct) {
				s.PUint16 = aws.Uint16(24)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PUint16 = aws.Uint16(42)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PUint16 = aws.Uint16(42)
			},
		},
		"PUint16 explicitly overridden by zero value": {
			original: func(s *testBasicTransformerStruct) {
				s.PUint16 = aws.Uint16(24)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PUint16 = aws.Uint16(0)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PUint16 = aws.Uint16(0)
			},
		},
		"PUint16 not overridden by nil": {
			original: func(s *testBasicTransformerStruct) {
				s.PUint16 = aws.Uint16(24)
			},
			override: func(s *testBasicTransformerStruct) {},
			wanted: func(s *testBasicTransformerStruct) {
				s.PUint16 = aws.Uint16(24)
			},
		},
		"PUint32 overridden": {
			original: func(s *testBasicTransformerStruct) {
				s.PUint32 = aws.Uint32(24)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PUint32 = aws.Uint32(42)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PUint32 = aws.Uint32(42)
			},
		},
		"PUint32 explicitly overridden by zero value": {
			original: func(s *testBasicTransformerStruct) {
				s.PUint32 = aws.Uint32(24)
			},
			override: func(s *testBasicTransformerStruct) {
				s.PUint32 = aws.Uint32(0)
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PUint32 = aws.Uint32(0)
			},
		},
		"PUint32 not overridden by nil": {
			original: func(s *testBasicTransformerStruct) {
				s.PUint32 = aws.Uint32(24)
			},
			override: func(s *testBasicTransformerStruct) {},
			wanted: func(s *testBasicTransformerStruct) {
				s.PUint32 = aws.Uint32(24)
			},
		},
		"PString overridden": {
			original: func(s *testBasicTransformerStruct) {
				s.PString = aws.String("horse")
			},
			override: func(s *testBasicTransformerStruct) {
				s.PString = aws.String("pony")
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PString = aws.String("pony")
			},
		},
		"PString explicitly overridden by zero value": {
			original: func(s *testBasicTransformerStruct) {
				s.PString = aws.String("horse")
			},
			override: func(s *testBasicTransformerStruct) {
				s.PString = aws.String("")
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PString = aws.String("")
			},
		},
		"PString not overridden by nil": {
			original: func(s *testBasicTransformerStruct) {
				s.PString = aws.String("horse")
			},
			override: func(s *testBasicTransformerStruct) {},
			wanted: func(s *testBasicTransformerStruct) {
				s.PString = aws.String("horse")
			},
		},
		"PSlice overridden": {
			original: func(s *testBasicTransformerStruct) {
				mockSlice := []string{"horses", "run"}
				s.PSlice = &mockSlice
			},
			override: func(s *testBasicTransformerStruct) {
				mockSlice := []string{"pony", "run"}
				s.PSlice = &mockSlice
			},
			wanted: func(s *testBasicTransformerStruct) {
				mockSlice := []string{"pony", "run"}
				s.PSlice = &mockSlice
			},
		},
		"PSlice explicitly overridden by zero value": {
			original: func(s *testBasicTransformerStruct) {
				mockSlice := []string{"horses", "run"}
				s.PSlice = &mockSlice
			},
			override: func(s *testBasicTransformerStruct) {
				var mockSlice []string
				s.PSlice = &mockSlice
			},
			wanted: func(s *testBasicTransformerStruct) {
				var mockSlice []string
				s.PSlice = &mockSlice
			},
		},
		"PSlice not overridden by nil": {
			original: func(s *testBasicTransformerStruct) {
				mockSlice := []string{"horses", "run"}
				s.PSlice = &mockSlice
			},
			override: func(s *testBasicTransformerStruct) {},
			wanted: func(s *testBasicTransformerStruct) {
				mockSlice := []string{"horses", "run"}
				s.PSlice = &mockSlice
			},
		},
		"slice overridden": {
			original: func(s *testBasicTransformerStruct) {
				s.Slice = []string{"horses", "run"}
			},
			override: func(s *testBasicTransformerStruct) {
				s.Slice = []string{"pony", "run"}
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.Slice = []string{"pony", "run"}
			},
		},
		"slice explicitly overridden by zero value": {
			original: func(s *testBasicTransformerStruct) {
				s.Slice = []string{"horses", "run"}
			},
			override: func(s *testBasicTransformerStruct) {
				s.Slice = []string{}
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.Slice = []string{}
			},
		},
		"slice not overridden by nil": {
			original: func(s *testBasicTransformerStruct) {
				s.Slice = []string{"horses", "run"}
			},
			override: func(s *testBasicTransformerStruct) {
				s.Slice = nil
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.Slice = []string{"horses", "run"}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted testBasicTransformerStruct

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			err := mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(basicTransformer{}))

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}
