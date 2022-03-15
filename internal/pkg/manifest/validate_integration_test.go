//go:build integration || localintegration
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
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
	types = append(types, reflect.TypeOf(time.Duration(0)).String())
	types = append(types, reflect.TypeOf(yaml.Node{}).String())
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
			mft: &manifest.BackendService{},
		},
		"load balanced web service": {
			mft: &manifest.LoadBalancedWebService{},
		},
		"request-driven web service": {
			mft: &manifest.RequestDrivenWebService{},
		},
		"schedule job": {
			mft: &manifest.ScheduledJob{},
		},
		"worker service": {
			mft: &manifest.WorkerService{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := isValid(reflect.ValueOf(tc.mft).Type())
			require.NoError(t, err)
		})
	}
}

func isValid(typ reflect.Type) error {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	// Skip if it is a type that doesn't need to implement Validate().
	for _, k := range basicTypesString() {
		if typ.String() == k {
			return nil
		}
	}
	if typ.Kind() == reflect.Interface {
		return nil
	}
	// For slice and map, validate its member type.
	if typ.Kind() == reflect.Array || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map {
		if err := isValid(typ.Elem()); err != nil {
			return err
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
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		// Skip private fields.
		if !field.IsExported() {
			continue
		}
		if err := isValid(field.Type); err != nil {
			return err
		}
	}
	return nil
}
