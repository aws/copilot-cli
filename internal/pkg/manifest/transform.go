// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/imdario/mergo"
)

var fmtExclusiveFieldsSpecifiedTogether = "invalid manifest: %s %s mutually exclusive with %s and shouldn't be specified at the same time"

var defaultTransformers = []mergo.Transformers{
	// NOTE: basicTransformer needs to be used before the rest of the custom transformers, because the other transformers
	// do not merge anything - they just unset the fields that do not get specified in source manifest.
	basicTransformer{},
	imageTransformer{},
	buildArgsOrStringTransformer{},
	aliasTransformer{},
	stringSliceOrStringTransformer{},
	platformArgsOrStringTransformer{},
	securityGroupsIDsOrConfigTransformer{},
	serviceConnectTransformer{},
	placementArgOrStringTransformer{},
	subnetListOrArgsTransformer{},
	unionTransformer{},
	countTransformer{},
	advancedCountTransformer{},
	scalingConfigOrTTransformer[Percentage]{},
	scalingConfigOrTTransformer[int]{},
	scalingConfigOrTTransformer[time.Duration]{},
	rangeTransformer{},
	efsConfigOrBoolTransformer{},
	efsVolumeConfigurationTransformer{},
	sqsQueueOrBoolTransformer{},
	httpOrBoolTransformer{},
	secretTransformer{},
	environmentCDNConfigTransformer{},
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

type aliasTransformer struct{}

// Transformer returns custom merge logic for Alias's fields.
func (t aliasTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(Alias{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct := dst.Convert(reflect.TypeOf(Alias{})).Interface().(Alias)
		srcStruct := src.Convert(reflect.TypeOf(Alias{})).Interface().(Alias)

		if len(srcStruct.AdvancedAliases) != 0 {
			dstStruct.StringSliceOrString = StringSliceOrString{}
		}

		if !srcStruct.StringSliceOrString.isEmpty() {
			dstStruct.AdvancedAliases = nil
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct).Convert(typ))
		}
		return nil
	}
}

type stringSliceOrStringTransformer struct{}

// Transformer returns custom merge logic for StringSliceOrString's fields.
func (t stringSliceOrStringTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if !typ.ConvertibleTo(reflect.TypeOf(StringSliceOrString{})) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct := dst.Convert(reflect.TypeOf(StringSliceOrString{})).Interface().(StringSliceOrString)
		srcStruct := src.Convert(reflect.TypeOf(StringSliceOrString{})).Interface().(StringSliceOrString)

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

type securityGroupsIDsOrConfigTransformer struct{}

// Transformer returns custom merge logic for SecurityGroupsIDsOrConfig's fields.
func (s securityGroupsIDsOrConfigTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(SecurityGroupsIDsOrConfig{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(SecurityGroupsIDsOrConfig), src.Interface().(SecurityGroupsIDsOrConfig)

		if !srcStruct.AdvancedConfig.isEmpty() {
			dstStruct.IDs = nil
		}

		if srcStruct.IDs != nil {
			dstStruct.AdvancedConfig = SecurityGroupsConfig{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type placementArgOrStringTransformer struct{}

// Transformer returns custom merge logic for placementArgOrString's fields.
func (t placementArgOrStringTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(PlacementArgOrString{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(PlacementArgOrString), src.Interface().(PlacementArgOrString)

		if srcStruct.PlacementString != nil {
			dstStruct.PlacementArgs = PlacementArgs{}
		}

		if !srcStruct.PlacementArgs.isEmpty() {
			dstStruct.PlacementString = nil
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type serviceConnectTransformer struct{}

// Transformer returns custom merge logic for serviceConnectTransformer's fields.
func (t serviceConnectTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(ServiceConnectBoolOrArgs{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(ServiceConnectBoolOrArgs), src.Interface().(ServiceConnectBoolOrArgs)

		if srcStruct.EnableServiceConnect != nil {
			dstStruct.ServiceConnectArgs = ServiceConnectArgs{}
		}

		if !srcStruct.ServiceConnectArgs.isEmpty() {
			dstStruct.EnableServiceConnect = nil
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type subnetListOrArgsTransformer struct{}

// Transformer returns custom merge logic for subnetListOrArgsTransformer's fields.
func (t subnetListOrArgsTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(SubnetListOrArgs{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(SubnetListOrArgs), src.Interface().(SubnetListOrArgs)

		if len(srcStruct.IDs) != 0 {
			dstStruct.SubnetArgs = SubnetArgs{}
		}

		if !srcStruct.SubnetArgs.isEmpty() {
			dstStruct.IDs = nil
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type unionTransformer struct{}

var unionPrefix, _, _ = strings.Cut(reflect.TypeOf(Union[any, any]{}).String(), "[")

// Transformer returns custom merge logic for union types.
func (t unionTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	// :sweat_smile: https://github.com/golang/go/issues/54393
	// reflect currently doesn't have support for getting type parameters
	// or checking if a type is a non-specific instantiation of a generic type
	// (i.e., no way to tell if the type Union[string, bool] is a Union)
	isUnion := strings.HasPrefix(typ.String(), unionPrefix)
	if !isUnion {
		return nil
	}

	return func(dst, src reflect.Value) (err error) {
		defer func() {
			// should realistically never happen unless Union type code has been
			// refactored to change functions called via reflection.
			if r := recover(); r != nil {
				err = fmt.Errorf("override union: %v", r)
			}
		}()

		isBasic := src.MethodByName("IsBasic").Call(nil)[0].Bool()
		isAdvanced := src.MethodByName("IsAdvanced").Call(nil)[0].Bool()

		// Call SetType with the correct type based on src's type.
		// We use the value from dst because it holds the merged value.
		if isBasic {
			if dst.CanAddr() {
				dst.Addr().MethodByName("SetBasic").Call([]reflect.Value{dst.FieldByName("Basic")})
			}
		} else if isAdvanced {
			if dst.CanAddr() {
				dst.Addr().MethodByName("SetAdvanced").Call([]reflect.Value{dst.FieldByName("Advanced")})
			}
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

type scalingConfigOrTTransformer[T ~int | time.Duration] struct{}

// Transformer returns custom merge logic for ScalingConfigOrPercentage's fields.
func (t scalingConfigOrTTransformer[T]) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(ScalingConfigOrT[T]{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(ScalingConfigOrT[T]), src.Interface().(ScalingConfigOrT[T])

		if !srcStruct.ScalingConfig.IsEmpty() {
			dstStruct.Value = nil
		}

		if srcStruct.Value != nil {
			dstStruct.ScalingConfig = AdvancedScalingConfig[T]{}
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

type sqsQueueOrBoolTransformer struct{}

// Transformer returns custom merge logic for SQSQueueOrBool's fields.
func (q sqsQueueOrBoolTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(SQSQueueOrBool{}) {
		return nil
	}
	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(SQSQueueOrBool), src.Interface().(SQSQueueOrBool)

		if !srcStruct.Advanced.IsEmpty() {
			dstStruct.Enabled = nil
		}

		if srcStruct.Enabled != nil {
			dstStruct.Advanced = SQSQueue{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type httpOrBoolTransformer struct{}

// Transformer returns custom merge logic for HTTPOrBool's fields.
func (t httpOrBoolTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(HTTPOrBool{}) {
		return nil
	}
	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(HTTPOrBool), src.Interface().(HTTPOrBool)

		if !srcStruct.HTTP.IsEmpty() {
			dstStruct.Enabled = nil
		}

		if srcStruct.Enabled != nil {
			dstStruct.HTTP = HTTP{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type secretTransformer struct{}

// Transformer returns custom merge logic for Secret's fields.
func (t secretTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(Secret{}) {
		return nil
	}
	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(Secret), src.Interface().(Secret)

		if !srcStruct.fromSecretsManager.IsEmpty() {
			dstStruct.from = StringOrFromCFN{}
		}

		if !srcStruct.from.isEmpty() {
			dstStruct.fromSecretsManager = secretsManagerSecret{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type environmentCDNConfigTransformer struct{}

// Transformer returns custom merge logic for environmentCDNConfig's fields.
func (t environmentCDNConfigTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf(EnvironmentCDNConfig{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		dstStruct, srcStruct := dst.Interface().(EnvironmentCDNConfig), src.Interface().(EnvironmentCDNConfig)

		if !srcStruct.Config.isEmpty() {
			dstStruct.Enabled = nil
		}

		if srcStruct.Enabled != nil {
			dstStruct.Config = AdvancedCDNConfig{}
		}

		if dst.CanSet() { // For extra safety to prevent panicking.
			dst.Set(reflect.ValueOf(dstStruct))
		}
		return nil
	}
}

type basicTransformer struct{}

// Transformer returns custom merge logic for volume's fields.
func (t basicTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ.Kind() == reflect.Slice {
		return transformPBasicOrSlice
	}

	if typ.Kind() == reflect.Ptr {
		for _, k := range basicKinds {
			if typ.Elem().Kind() == k {
				return transformPBasicOrSlice
			}
		}
	}
	return nil
}

func transformPBasicOrSlice(dst, src reflect.Value) error {
	// This condition shouldn't ever be true. It's merely here for extra safety so that `src.IsNil` won't panic.
	if src.Kind() != reflect.Ptr && src.Kind() != reflect.Slice {
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
