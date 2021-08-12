// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"reflect"

	"github.com/imdario/mergo"
)

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
	if transform := transformBasic(typ); transform != nil {
		return transform
	}

	if typ == reflect.TypeOf(Image{}) {
		return transformImage()
	}

	if typ.String() == "map[string]manifest.Volume" {
		return transformMapStringToVolume()
	}
	return nil
}

type volumeTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t volumeTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	return transformBasic(typ)
}

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
		dst.Set(src)
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

			// Perform default merge
			dstV := dstElement.Interface().(Volume)
			srcV := srcElement.Interface().(Volume)
			err := mergo.Merge(&dstV, srcV, mergo.WithOverride, mergo.WithTransformers(volumeTransformer{}))
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

// transformImage implements customized merge logic for Image field of manifest.
// It merges `DockerLabels` and `DependsOn` in the default manager (i.e. with configurations mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
// And then overrides both `Build` and `Location` fields at the same time with the src values, given that they are non-empty themselves.
func transformImage() func(dst, src reflect.Value) error {
	return func(dst, src reflect.Value) error {
		// Perform default merge
		dstImage := dst.Interface().(Image)
		srcImage := src.Interface().(Image)

		err := mergo.Merge(&dstImage, srcImage, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
		if err != nil {
			return err
		}

		// Perform customized merge
		dstBuild := dst.FieldByName("Build")
		dstLocation := dst.FieldByName("Location")

		srcBuild := src.FieldByName("Build")
		srcLocation := src.FieldByName("Location")

		if !srcBuild.IsZero() || !srcLocation.IsZero() {
			dstBuild.Set(srcBuild)
			dstLocation.Set(srcLocation)
		}
		return nil
	}
}
