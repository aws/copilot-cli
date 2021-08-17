// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"reflect"

	"github.com/dustin/go-humanize/english"

	"github.com/imdario/mergo"
)

// See a complete list of `reflect.Kind` here: https://pkg.go.dev/reflect#Kind.
var basicKinds = []reflect.Kind{
	reflect.Bool,
	reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
	reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
	reflect.Float32, reflect.Float64,
	reflect.Complex64, reflect.Complex128,
	reflect.Array, reflect.String, reflect.Slice,
}
var fmtExclusiveFieldsSpecifiedTogether = "invalid manifest: %s %s mutually exclusive with %s and shouldn't be specified at the same time"

type workloadTransformer struct{}

// Transformer returns custom merge logic for workload's fields.
func (t workloadTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if transform := t.transformExclusiveTypes(typ); transform != nil {
		return transform
	}

	if transform := t.transformCompositeTypes(typ); transform != nil {
		return transform
	}

	if typ.String() == "map[string]manifest.Volume" {
		return transformMapStringToVolume
	}

	if transform := transformBasic(typ); transform != nil {
		return transform
	}

	return nil
}

func (t workloadTransformer) transformCompositeTypes(typ reflect.Type) func(dst, src reflect.Value) error {
	var et exclusiveTransformer
	switch typ {
	case reflect.TypeOf(EntryPointOverride{}), reflect.TypeOf(CommandOverride{}), reflect.TypeOf(Alias{}):
		et.merge = func(dst, src reflect.Value) error {
			dstStruct := dst.Convert(reflect.TypeOf(stringSliceOrString{})).Interface().(stringSliceOrString)
			srcStruct := src.Convert(reflect.TypeOf(stringSliceOrString{})).Interface().(stringSliceOrString)
			if err := mergo.Merge(&dstStruct, srcStruct, opts(basicTransformer{})...); err != nil {
				return err
			}
			dst.Set(reflect.ValueOf(dstStruct).Convert(typ))
			return nil
		}
		et.resetExclusiveFields = func(dst, src reflect.Value) error {
			return resetExclusiveFields(dst, src, []string{"String"}, []string{"StringSlice"})
		}
	case reflect.TypeOf(PlatformArgsOrString{}):
		et.merge = func(dst, src reflect.Value) error {
			dstPlatformArgsOrString := dst.Interface().(PlatformArgsOrString)
			srcPlatformArgsOrString := src.Interface().(PlatformArgsOrString)
			if err := mergo.Merge(&dstPlatformArgsOrString, srcPlatformArgsOrString, opts(basicTransformer{})...); err != nil {
				return err
			}
			dst.Set(reflect.ValueOf(dstPlatformArgsOrString))
			return nil
		}
		et.resetExclusiveFields = func(dst, src reflect.Value) error {
			return resetExclusiveFields(dst, src, []string{"PlatformString"}, []string{"PlatformArgs"})
		}
	case reflect.TypeOf(HealthCheckArgsOrString{}):
		et.merge = func(dst, src reflect.Value) error {
			dstHC := dst.Interface().(HealthCheckArgsOrString)
			srcHC := src.Interface().(HealthCheckArgsOrString)
			if err := mergo.Merge(&dstHC, srcHC, opts(basicTransformer{})...); err != nil {
				return err
			}
			dst.Set(reflect.ValueOf(dstHC))
			return nil
		}
		et.resetExclusiveFields = func(dst, src reflect.Value) error {
			return resetExclusiveFields(dst, src, []string{"HealthCheckPath"}, []string{"HealthCheckArgs"})
		}
	case reflect.TypeOf(Count{}):
		et.merge = func(dst, src reflect.Value) error {
			dstC := dst.Interface().(Count)
			srcC := src.Interface().(Count)
			if err := mergo.Merge(&dstC, srcC, opts(countTransformer{})...); err != nil {
				return err
			}
			dst.Set(reflect.ValueOf(dstC))
			return nil
		}
		et.resetExclusiveFields = func(dst, src reflect.Value) error {
			return resetExclusiveFields(dst, src, []string{"Value"}, []string{"AdvancedCount"})
		}
	default:
		return nil // Return `nil` if this is not a composite type.
	}
	return et.transform
}

func (t workloadTransformer) transformExclusiveTypes(originalType reflect.Type) func(dst, src reflect.Value) error {
	var et exclusiveTransformer
	switch originalType {
	case reflect.TypeOf(Image{}):
		et.merge = func(dst, src reflect.Value) error {
			dstImage := dst.Interface().(Image)
			srcImage := src.Interface().(Image)
			if err := mergo.Merge(&dstImage, srcImage, opts(imageTransformer{})...); err != nil {
				return err
			}
			dst.Set(reflect.ValueOf(dstImage))
			return nil
		}
		et.resetExclusiveFields = func(dst, src reflect.Value) error {
			return resetExclusiveFields(dst, src, []string{"Build"}, []string{"Location"})
		}
	default:
		return nil
	}
	return et.transform
}

type imageTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t imageTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if transform := t.transformCompositeTypes(typ); transform != nil {
		return transform
	}
	return transformBasic(typ)
}

func (t imageTransformer) transformCompositeTypes(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(BuildArgsOrString{}) {
		return nil
	}

	var et exclusiveTransformer
	et.merge = func(dst, src reflect.Value) error {
		dstBuildArgsOrString := dst.Interface().(BuildArgsOrString)
		srcBuildArgsOrString := src.Interface().(BuildArgsOrString)
		if err := mergo.Merge(&dstBuildArgsOrString, srcBuildArgsOrString, opts(basicTransformer{})...); err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(dstBuildArgsOrString))
		return nil
	}
	et.resetExclusiveFields = func(dst, src reflect.Value) error {
		return resetExclusiveFields(dst, src, []string{"BuildString"}, []string{"BuildArgs"})
	}
	return et.transform
}

type volumeTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t volumeTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if transform := t.transformCompositeTypes(typ); transform != nil {
		return transform
	}
	return transformBasic(typ)
}

func (t volumeTransformer) transformCompositeTypes(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(EFSConfigOrBool{}) {
		return nil
	}
	var et exclusiveTransformer
	et.merge = func(dst, src reflect.Value) error {
		dstEFSConfigOrBool := dst.Interface().(EFSConfigOrBool)
		srcEFSConfigOrBool := src.Interface().(EFSConfigOrBool)
		if err := mergo.Merge(&dstEFSConfigOrBool, srcEFSConfigOrBool, opts(efsConfigOrBoolTransformer{})...); err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(dstEFSConfigOrBool))
		return nil
	}
	et.resetExclusiveFields = func(dst, src reflect.Value) error {
		return resetExclusiveFields(dst, src, []string{"Enabled"}, []string{"Advanced"})
	}
	return et.transform
}

type efsConfigOrBoolTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t efsConfigOrBoolTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if transform := t.transformExclusiveTypes(typ); transform != nil {
		return transform
	}
	return transformBasic(typ)
}

func (t efsConfigOrBoolTransformer) transformExclusiveTypes(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(EFSVolumeConfiguration{}) {
		return nil
	}

	var et exclusiveTransformer
	et.merge = func(dst, src reflect.Value) error {
		dstV := dst.Interface().(EFSVolumeConfiguration)
		srcV := src.Interface().(EFSVolumeConfiguration)
		if err := mergo.Merge(&dstV, srcV, opts(volumeTransformer{})...); err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(dstV))
		return nil
	}
	et.resetExclusiveFields = func(dst, src reflect.Value) error {
		return resetExclusiveFields(dst, src, []string{"UID", "GID"}, []string{"FileSystemID", "RootDirectory", "AuthConfig"})
	}
	return et.transform
}

type countTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t countTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if transform := t.transformExclusiveTypes(typ); transform != nil {
		return transform
	}
	return transformBasic(typ)
}

func (t countTransformer) transformExclusiveTypes(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(AdvancedCount{}) {
		return nil
	}
	var et exclusiveTransformer
	et.merge = func(dst, src reflect.Value) error {
		dstC := dst.Interface().(AdvancedCount)
		srcC := src.Interface().(AdvancedCount)
		if err := mergo.Merge(&dstC, srcC, opts(advancedCountTransformer{})...); err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(dstC))
		return nil
	}
	et.resetExclusiveFields = func(dst, src reflect.Value) error {
		return resetExclusiveFields(dst, src, []string{"Spot"}, []string{"Range", "CPU", "Memory", "Requests", "ResponseTime"})
	}
	return et.transform
}

type advancedCountTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t advancedCountTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if transform := t.transformCompositeTypes(typ); transform != nil {
		return transform
	}
	return transformBasic(typ)
}

func (t advancedCountTransformer) transformCompositeTypes(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(Range{}) {
		return nil
	}

	var et exclusiveTransformer
	et.merge = func(dst, src reflect.Value) error {
		dstHC := dst.Interface().(Range)
		srcHC := src.Interface().(Range)
		if err := mergo.Merge(&dstHC, srcHC, opts(basicTransformer{})...); err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(dstHC))
		return nil
	}
	et.resetExclusiveFields = func(dst, src reflect.Value) error {
		return resetExclusiveFields(dst, src, []string{"Value"}, []string{"RangeConfig"})
	}
	return et.transform
}

// transformBasic implements customized merge logic for manifest fields that are number, string, bool, array, and duration.
func transformBasic(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ.Kind() == reflect.Slice {
		return transformSlice
	}

	if typ.Kind() == reflect.Ptr {
		for _, k := range basicKinds {
			if typ.Elem().Kind() == k {
				return transformPBasic()
			}
		}

		if typ.Elem().Kind() == reflect.Struct {
			return transformPStruct()
		}
	}
	return nil
}

func transformSlice(dst, src reflect.Value) error {
	if !src.IsNil() {
		dst.Set(src)
	}
	return nil
}

func transformMapStringToVolume(dst, src reflect.Value) error {
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
		if err := mergo.Merge(&dstV, srcV, opts(volumeTransformer{})...); err != nil {
			return err
		}

		// Set merged value for the key
		dst.SetMapIndex(key, reflect.ValueOf(dstV))
	}
	return nil
}

func transformPStruct() func(dst, src reflect.Value) error {
	return func(dst, src reflect.Value) error {
		if src.IsNil() {
			return nil
		}

		// TODO: these can be removed if we change all pointers struct to struct
		// Perform default merge
		var err error
		switch dst.Elem().Type().Name() {
		case "ContainerHealthCheck":
			dstElem := dst.Interface().(*ContainerHealthCheck)
			srcElem := src.Elem().Interface().(ContainerHealthCheck)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "PlatformArgsOrString":
			dstElem := dst.Interface().(*PlatformArgsOrString)
			srcElem := src.Elem().Interface().(PlatformArgsOrString)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "EntryPointOverride":
			dstElem := dst.Interface().(*EntryPointOverride)
			srcElem := src.Elem().Interface().(EntryPointOverride)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "CommandOverride":
			dstElem := dst.Interface().(*CommandOverride)
			srcElem := src.Elem().Interface().(CommandOverride)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "NetworkConfig":
			dstElem := dst.Interface().(*NetworkConfig)
			srcElem := src.Elem().Interface().(NetworkConfig)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "vpcConfig":
			dstElem := dst.Interface().(*vpcConfig)
			srcElem := src.Elem().Interface().(vpcConfig)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "Logging":
			dstElem := dst.Interface().(*Logging)
			srcElem := src.Elem().Interface().(Logging)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "Storage":
			dstElem := dst.Interface().(*Storage)
			srcElem := src.Elem().Interface().(Storage)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "Alias":
			dstElem := dst.Interface().(*Alias)
			srcElem := src.Elem().Interface().(Alias)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "Range":
			dstElem := dst.Interface().(*Range)
			srcElem := src.Elem().Interface().(Range)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(advancedCountTransformer{}))
		case "SidecarConfig":
			dstElem := dst.Interface().(*SidecarConfig)
			srcElem := src.Elem().Interface().(SidecarConfig)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "SubscribeConfig":
			dstElem := dst.Interface().(*SubscribeConfig)
			srcElem := src.Elem().Interface().(SubscribeConfig)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "SQSQueue":
			dstElem := dst.Interface().(*SQSQueue)
			srcElem := src.Elem().Interface().(SQSQueue)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		case "EFSConfigOrBool":
			dstElem := dst.Interface().(*EFSConfigOrBool)
			srcElem := src.Elem().Interface().(EFSConfigOrBool)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(volumeTransformer{}))
		case "AuthorizationConfig":
			dstElem := dst.Interface().(*AuthorizationConfig)
			srcElem := src.Elem().Interface().(AuthorizationConfig)
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
		}

		if err != nil {
			return err
		}
		return nil
	}
}

func transformPBasic() func(dst, src reflect.Value) error {
	return func(dst, src reflect.Value) error {
		if src.IsNil() {
			return nil
		}
		dst.Set(src)
		return nil
	}
}

type exclusiveTransformer struct {
	merge                func(dst, src reflect.Value) error
	resetExclusiveFields func(dst, src reflect.Value) error
}

func (t exclusiveTransformer) transform(dst, src reflect.Value) error {
	// Perform default merge
	if err := t.merge(dst, src); err != nil {
		return err
	}
	// Set or unset exclusive fields
	if err := t.resetExclusiveFields(dst, src); err != nil {
		return err
	}
	return nil
}

type basicTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t basicTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	return transformBasic(typ)
}

func opts(transformers mergo.Transformers) []func(*mergo.Config) {
	return []func(*mergo.Config){
		mergo.WithOverride,
		mergo.WithTransformers(transformers),
	}
}

func resetExclusiveFields(dst, src reflect.Value, fieldsA, fieldsB []string) error {
	var fieldsASpecifiedInSrc, fieldsBSpecifiedInSrc bool
	for _, a := range fieldsA {
		if !src.FieldByName(a).IsZero() {
			fieldsASpecifiedInSrc = true
			break
		}
	}

	for _, b := range fieldsB {
		if !src.FieldByName(b).IsZero() {
			fieldsBSpecifiedInSrc = true
			break
		}
	}

	// TODO: we should validate that they are not specified at the same time at an earlier stage, possibly in `manifest.validate`.
	if fieldsASpecifiedInSrc && fieldsBSpecifiedInSrc {
		return fmt.Errorf(fmtExclusiveFieldsSpecifiedTogether,
			english.WordSeries(fieldsA, "and"),
			english.Plural(len(fieldsA), "is", "are"),
			english.WordSeries(fieldsB, "and"))
	}

	if fieldsASpecifiedInSrc {
		for _, b := range fieldsB {
			dst.FieldByName(b).Set(reflect.Zero(dst.FieldByName(b).Type()))
		}
	}

	if fieldsBSpecifiedInSrc {
		for _, a := range fieldsA {
			dst.FieldByName(a).Set(reflect.Zero(dst.FieldByName(a).Type()))
		}
	}

	return nil
}
