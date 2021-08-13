// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"reflect"

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

type workloadTransformer struct{}

// Transformer returns custom merge logic for workload's fields.
func (t workloadTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(Image{}) {
		return transformImage()
	}

	if typ.String() == "map[string]manifest.Volume" {
		return transformMapStringToVolume()
	}

	if transform := transformBasic(typ); transform != nil {
		return transform
	}
	return nil
}

type basicTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t basicTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	return transformBasic(typ)
}

type imageTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t imageTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(BuildArgsOrString{}) {
		return transformBuildArgsOrString()
	}
	return transformBasic(typ)
}

// transformBasic implements customized merge logic for manifest fields that are number, string, bool, array, and duration.
func transformBasic(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ.Kind() == reflect.Slice {
		return transformSlice()
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

func transformSlice() func(dst, src reflect.Value) error {
	return func(dst, src reflect.Value) error {
		if !src.IsNil() {
			dst.Set(src)
		}
		return nil
	}
}

func transformMapStringToVolume() func(dst, src reflect.Value) error {
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
			err := mergo.Merge(&dstV, srcV, mergo.WithOverride, mergo.WithTransformers(basicTransformer{}))
			if err != nil {
				return err
			}

			// Set merged value for the key
			dst.SetMapIndex(key, reflect.ValueOf(dstV))
		}
		return nil
	}
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
			err = mergo.Merge(dstElem, srcElem, mergo.WithOverride, mergo.WithTransformers(workloadTransformer{}))
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

func transformBuildArgsOrString() func(dst, src reflect.Value) error {
	return func(dst, src reflect.Value) error {
		// Perform default merge
		dstBuildArgsOrString := dst.Interface().(BuildArgsOrString)
		srcBuildArgsOrString := src.Interface().(BuildArgsOrString)

		err := mergo.Merge(&dstBuildArgsOrString, srcBuildArgsOrString, mergo.WithOverride, mergo.WithTransformers(basicTransformer{}))
		if err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(dstBuildArgsOrString))

		// Perform customized merge
		dstString := dst.FieldByName("BuildString")
		dstArgs := dst.FieldByName("BuildArgs")

		srcString := src.FieldByName("BuildString")
		srcArgs := src.FieldByName("BuildArgs")

		//` `srcArgs.IsZero()` and `srcString.IsZero()` shouldn't return true at the same time if the manifest is not malformed.
		if !srcArgs.IsZero() {
			dstString.Set(srcString)
		} else if !srcString.IsNil() {
			dstArgs.Set(srcArgs)
		}

		return nil
	}
}

// transformImage implements customized merge logic for Image field of manifest.
// It merges `DockerLabels` and `DependsOn` in the default manager (i.e. with configurations mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
// And then overrides both `Build` and `Location` fields at the same time with the src values, given that they are non-empty themselves.
func transformImage() func(dst, src reflect.Value) error {
	return func(dst, src reflect.Value) error {
		// Perform default merge
		dstImage := dst.Interface().(Image)
		srcImage := src.Interface().(Image)

		err := mergo.Merge(&dstImage, srcImage, mergo.WithOverride, mergo.WithTransformers(imageTransformer{}))
		if err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(dstImage))

		// Perform customized merge
		dstBuild := dst.FieldByName("Build")
		dstLocation := dst.FieldByName("Location")

		srcBuild := src.FieldByName("Build")
		srcLocation := src.FieldByName("Location")

		//` `srcBuild.IsZero()` and `srcLocation.IsZero()` shouldn't return true at the same time if the manifest is not malformed.
		if !srcBuild.IsZero() {
			dstLocation.Set(srcLocation)
		} else if !srcLocation.IsZero() {
			dstBuild.Set(srcBuild)
		}

		return nil
	}
}
