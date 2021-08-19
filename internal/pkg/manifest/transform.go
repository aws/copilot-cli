// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"reflect"

	"github.com/imdario/mergo"
)

var fmtExclusiveFieldsSpecifiedTogether = "invalid manifest: %s %s mutually exclusive with %s and shouldn't be specified at the same time"

var defaultTransformers = []mergo.Transformers{
	// NOTE: mapToVolumeTransformer need has to be the first transformer. Otherwise `mergo` will overwrite the `dst` map
	// completely and we will lose `dst`'s values.
	mapToVolumeTransformer{},

	// NOTE: basicTransformer needs to used before the rest of the custom transformers, because the other transformers
	// do not merge anything - they just unset the fields that do not get specified in source manifest.
	basicTransformer{},
	imageTransformer{},
	buildArgsOrStringTransformer{},
	stringSliceOrStringTransformer{},
	platformArgsOrStringTransformer{},
	healthCheckArgsOrStringTransformer{},
	countTransformer{},
	advancedCountTransformer{},
	rangeTransformer{},
}

// See a complete list of `reflect.Kind` here: https://pkg.go.dev/reflect#Kind.
var basicKinds = []reflect.Kind{
	reflect.Bool,
	reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
	reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
	reflect.Float32, reflect.Float64,
	reflect.Complex64, reflect.Complex128,
	reflect.Array, reflect.String, reflect.Slice,
}

type imageTransformer struct{}

// Transformer provides custom logic to transform an Image.
func (t imageTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(Image{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(Image), src.Interface().(Image)

		if !srcStruct.Build.isEmpty() && srcStruct.Location != nil {
			return fmt.Errorf(fmtExclusiveFieldsSpecifiedTogether, "image.build", "is", "image.location")
		}

		if !srcStruct.Build.isEmpty() {
			dstStruct.Location = nil
		}

		if srcStruct.Location != nil {
			dstStruct.Build = BuildArgsOrString{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type buildArgsOrStringTransformer struct{}

// Transformer returns custom merge logic for BuildArgsOrString's fields.
func (t buildArgsOrStringTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(BuildArgsOrString{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(BuildArgsOrString), src.Interface().(BuildArgsOrString)

		if !srcStruct.BuildArgs.isEmpty() {
			dstStruct.BuildString = nil
		}

		if srcStruct.BuildString != nil {
			dstStruct.BuildArgs = DockerBuildArgs{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type stringSliceOrStringTransformer struct{}

// Transformer returns custom merge logic for stringSliceOrStringTransformer's fields.
func (t stringSliceOrStringTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if !typ.ConvertibleTo(reflect.TypeOf(stringSliceOrString{})) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct := dst.Convert(reflect.TypeOf(stringSliceOrString{})).Interface().(stringSliceOrString)
		srcStruct := src.Convert(reflect.TypeOf(stringSliceOrString{})).Interface().(stringSliceOrString)

		if srcStruct.String != nil {
			dstStruct.StringSlice = nil
		}

		if srcStruct.StringSlice != nil {
			dstStruct.String = nil
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct).Convert(typ))
		}
		return nil
	}
}

type platformArgsOrStringTransformer struct{}

// Transformer returns custom merge logic for PlatformArgsOrString's fields.
func (t platformArgsOrStringTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(PlatformArgsOrString{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(PlatformArgsOrString), src.Interface().(PlatformArgsOrString)

		if srcStruct.PlatformString != nil {
			dstStruct.PlatformArgs = PlatformArgs{}
		}

		if !srcStruct.PlatformArgs.isEmpty() {
			dstStruct.PlatformString = nil
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type healthCheckArgsOrStringTransformer struct{}

// Transformer returns custom merge logic for HealthCheckArgsOrString's fields.
func (t healthCheckArgsOrStringTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(HealthCheckArgsOrString{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(HealthCheckArgsOrString), src.Interface().(HealthCheckArgsOrString)

		if srcStruct.HealthCheckPath != nil {
			dstStruct.HealthCheckArgs = HTTPHealthCheckArgs{}
		}

		if !srcStruct.HealthCheckArgs.isEmpty() {
			dstStruct.HealthCheckPath = nil
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type countTransformer struct{}

// Transformer returns custom merge logic for Count's fields.
func (t countTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(Count{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(Count), src.Interface().(Count)

		if !srcStruct.AdvancedCount.IsEmpty() {
			dstStruct.Value = nil
		}

		if srcStruct.Value != nil {
			dstStruct.AdvancedCount = AdvancedCount{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type advancedCountTransformer struct{}

// Transformer returns custom merge logic for AdvancedCount's fields.
func (t advancedCountTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(AdvancedCount{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(AdvancedCount), src.Interface().(AdvancedCount)

		if srcStruct.Spot != nil {
			dstStruct.unsetAutoscaling()
		}

		if srcStruct.hasAutoscaling() {
			dstStruct.Spot = nil
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type rangeTransformer struct{}

// Transformer returns custom merge logic for Range's fields.
func (t rangeTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(Range{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(Range), src.Interface().(Range)

		if !srcStruct.RangeConfig.IsEmpty() {
			dstStruct.Value = nil
		}

		if srcStruct.Value != nil {
			dstStruct.RangeConfig = RangeConfig{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type mapToVolumeTransformer struct{}

// Transformer returns custom merge logic for map[string]Volume.
func (t mapToVolumeTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	/**
	There are two things to notice:
	1. Custom logic for `map[string]Volume` is needed because when `withOverride` flag is specified, `mergo` set
	dst map's value directly by src map's value, instead of continuing deep merging `Volume`.
	For example, given that
			dst = map[string]Volume{
				"volume1": {
					prop1: 1,
					prop2: 2,
				},
			}
	and 	src = map[string]Volume{
				"volume1": {
					prop1: 3,
				},
			}
	Without this transformer, the result would be
			res = map[string]Volume{
				"volume1": {
					prop1: 3,
				}
			}
	The merge stops at `map`, while we expect `Volume` to be merged instead of overwritten.
	2. Note that at the end the transformer sets src map's value as well by `src.SetMapIndex(key, reflect.ValueOf(dstV))`.
	This is because when `withOverride` flag is specified, `mergo` stops merging at map instead of `Volume`. The merged
	value will get overwritten by any other transformers.
	*/
	if typ.Kind() != reflect.Map {
		return nil
	}

	if typ.Key().Kind() != reflect.String || typ.Elem() != reflect.TypeOf(Volume{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		for _, key := range src.MapKeys() {
			srcElement := src.MapIndex(key)
			if !srcElement.IsValid() {
				continue
			}

			dstElement := dst.MapIndex(key)
			if !dstElement.IsValid() {
				dst.SetMapIndex(key, srcElement)
				continue
			}

			// Perform default merge
			dstV := dstElement.Interface().(Volume)
			srcV := srcElement.Interface().(Volume)

			transformers := []mergo.Transformers{
				efsConfigOrBoolTransformer{},
				efsVolumeConfigurationTransformer{},
				basicTransformer{},
			}

			for _, t := range transformers {
				if err := mergo.Merge(&dstV, srcV, mergo.WithOverride, mergo.WithTransformers(t)); err != nil {
					return err
				}
			}

			// Set merged value for the key
			dst.SetMapIndex(key, reflect.ValueOf(dstV))

			// NOTE: if we don't set the merged value for `src`, `dst`'s value for this key will be completely overwritten
			// (instead of merging) by other transformers.
			src.SetMapIndex(key, reflect.ValueOf(dstV))
		}
		return nil
	}
}

type efsConfigOrBoolTransformer struct{}

// Transformer returns custom merge logic for EFSConfigOrBool's fields.
func (t efsConfigOrBoolTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(EFSConfigOrBool{}) {
		return nil
	}
	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(EFSConfigOrBool), src.Interface().(EFSConfigOrBool)

		if !srcStruct.Advanced.IsEmpty() {
			dstStruct.Enabled = nil
		}

		if srcStruct.Enabled != nil {
			dstStruct.Advanced = EFSVolumeConfiguration{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type efsVolumeConfigurationTransformer struct{}

// Transformer returns custom merge logic for EFSVolumeConfiguration's fields.
func (t efsVolumeConfigurationTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(EFSVolumeConfiguration{}) {
		return nil
	}
	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(EFSVolumeConfiguration), src.Interface().(EFSVolumeConfiguration)
		if !srcStruct.EmptyUIDConfig() {
			dstStruct.unsetBYOConfig()
		}

		if !srcStruct.EmptyBYOConfig() {
			dstStruct.unsetUIDConfig()
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

func transformPBasic() func(dst, src reflect.Value) error {
	// NOTE: `dst` must be of kind reflect.Ptr.
	return func(dst, src reflect.Value) error {
		// This condition shouldn't ever be true. It's merely here for extra safety so that `src.IsNil` won't panic.
		if src.Kind() != reflect.Ptr {
			return nil
		}

		if src.IsNil() {
			return nil
		}

		if dst.CanSet() {
			dst.Set(src)
		}
		return nil
	}
}

type basicTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t basicTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ.Kind() == reflect.Slice {
		return func(dst, src reflect.Value) error {
			// This condition shouldn't ever be true. It's merely here for extra safety so that `src.IsNil` won't panic.
			if src.Kind() != reflect.Slice {
				return nil
			}

			if src.IsNil() {
				return nil
			}

			if dst.CanSet() {
				dst.Set(src)
			}

			return nil
		}
	}

	if typ.Kind() == reflect.Ptr {
		for _, k := range basicKinds {
			if typ.Elem().Kind() == k {
				return transformPBasic()
			}
		}
	}
	return nil
}
