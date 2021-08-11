package manifest

import (
	"reflect"

	"github.com/imdario/mergo"
)

type workloadTransformer struct{}

var overrideKinds = []reflect.Kind{
	reflect.Bool,
	reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
	reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
	reflect.Float32, reflect.Float64,
	reflect.Complex64, reflect.Complex128,
	reflect.Array, reflect.String, reflect.Slice,
}

// Transformer implements customized merge logic for Image field of manifest.
// It merges `DockerLabels` and `DependsOn` in the default manager (i.e. with configurations mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
// And then overrides both `Build` and `Location` fields at the same time with the src values, given that they are non-empty themselves.
func (t workloadTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(Image{}) {
		return transformImage()
	}

	if typ.Kind() == reflect.Ptr {
		for _, k := range overrideKinds {
			if typ.Elem().Kind() == k {
				return transformPointer() // Use `transformPointer` only if the pointer is to a "basic type" TODO: reword this
			}
		}
	}
	return nil
}

func transformPointer() func(dst, src reflect.Value) error {
	return func(dst, src reflect.Value) error {
		if src.IsNil() {
			return nil
		}
		dst.Set(src)
		return nil
	}
}

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
