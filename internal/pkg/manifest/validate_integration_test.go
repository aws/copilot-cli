// +build integration localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
)

var basicKinds = []reflect.Kind{
	reflect.Bool,
	reflect.String,
	reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
	reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
	reflect.Float32, reflect.Float64,
	reflect.Complex64, reflect.Complex128,
}

func basicTypesString() []string {
	var types []string
	for _, k := range basicKinds {
		types = append(types, k.String())
	}
	types = append(types, reflect.TypeOf(map[string]string{}).String())
	types = append(types, reflect.TypeOf([]string{}).String())
	return types
}

type validator interface {
	Validate() error
}

// Test_ValidateAudit ensures that every manifest struct implements "Validate()" method.
func Test_ValidateAudit(t *testing.T) {
	testCases := map[string]struct {
		mft manifest.WorkloadManifest
	}{
		"backend service": {
			mft: manifest.BackendService{},
		},
		"load balanced web service": {
			mft: manifest.LoadBalancedWebService{},
		},
		"request-driven web service": {
			mft: manifest.RequestDrivenWebService{},
		},
		"schedule job": {
			mft: manifest.ScheduledJob{},
		},
		"worker service": {
			mft: manifest.WorkerService{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := isValid(reflect.ValueOf(tc.mft))
			require.NoError(t, err)
		})
	}
}

func isValid(v reflect.Value) error {
	typ := v.Type()
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	// Skip if it is a type that doesn't need to implement Validate().
	if exceptionType(typ) {
		return nil
	}
	// For slice and map, validate individual member.
	if typ.Kind() == reflect.Array || typ.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			if err := isValid(v.Index(i)); err != nil {
				return err
			}
		}
		return nil
	}
	if typ.Kind() == reflect.Map {
		for _, k := range v.MapKeys() {
			if err := isValid(v.MapIndex(k)); err != nil {
				return err
			}
		}
		return nil
	}
	// Check if the field implements Validate().
	var val validator
	validatorType := reflect.TypeOf(&val).Elem()
	if !typ.Implements(validatorType) {
		return fmt.Errorf(`%v does not implement "Validate()"`, typ)
	}
	// For struct we'll check its members after its own validation.
	if typ.Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < v.NumField(); i++ {
		if err := isValid(v.Field(i)); err != nil {
			return err
		}
	}
	return nil
}

func exceptionType(typ reflect.Type) bool {
	for _, k := range basicTypesString() {
		if typ.String() == k {
			return true
		}
	}
	var tpl template.Parser
	if typ == reflect.TypeOf(&tpl).Elem() {
		return true
	}
	var duration time.Duration
	if typ == reflect.TypeOf(&duration).Elem() {
		return true
	}
	return false
}
