// +build integration localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
)

var basicKinds = []reflect.Kind{
	reflect.Bool,
	reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
	reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
	reflect.Float32, reflect.Float64,
	reflect.Complex64, reflect.Complex128,
	reflect.Array, reflect.String, reflect.Slice, reflect.Map,
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
	for _, k := range basicKinds {
		if typ.Kind() == k {
			return nil
		}
	}
	var val validator
	validatorType := reflect.TypeOf(&val).Elem()
	// template.Parser is not a manifest struct.
	var tpl template.Parser
	templaterType := reflect.TypeOf(&tpl).Elem()
	if !typ.Implements(templaterType) && !typ.Implements(validatorType) {
		return fmt.Errorf(`%v does not implement "Validate()"`, typ)
	}
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
