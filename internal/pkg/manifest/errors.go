// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudfront"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/dustin/go-humanize/english"
)

// ErrInvalidWorkloadType occurs when a user requested a manifest template type that doesn't exist.
type ErrInvalidWorkloadType struct {
	Type string
}

func (e *ErrInvalidWorkloadType) Error() string {
	return fmt.Sprintf("invalid manifest type: %s", e.Type)
}

// ErrInvalidPipelineManifestVersion occurs when the pipeline.yml/manifest.yml file
// contains invalid schema version during unmarshalling.
type ErrInvalidPipelineManifestVersion struct {
	invalidVersion PipelineSchemaMajorVersion
}

func (e *ErrInvalidPipelineManifestVersion) Error() string {
	return fmt.Sprintf("pipeline manifest contains invalid schema version: %d", e.invalidVersion)
}

// Is compares the 2 errors. Only returns true if the errors are of the same
// type and contain the same information.
func (e *ErrInvalidPipelineManifestVersion) Is(target error) bool {
	t, ok := target.(*ErrInvalidPipelineManifestVersion)
	return ok && t.invalidVersion == e.invalidVersion
}

// ErrUnknownProvider occurs CreateProvider() is called with configurations
// that do not map to any supported provider.
type ErrUnknownProvider struct {
	unknownProviderProperties interface{}
}

func (e *ErrUnknownProvider) Error() string {
	return fmt.Sprintf("no provider found for properties: %v",
		e.unknownProviderProperties)
}

// Is compares the 2 errors. Returns true if the errors are of the same
// type
func (e *ErrUnknownProvider) Is(target error) bool {
	_, ok := target.(*ErrUnknownProvider)
	return ok
}

type errFieldMustBeSpecified struct {
	missingField       string
	conditionalFields  []string
	allMustBeSpecified bool
}

func (e *errFieldMustBeSpecified) Error() string {
	errMsg := fmt.Sprintf(`%q must be specified`, e.missingField)
	if len(e.conditionalFields) == 0 {
		return errMsg
	}
	conjunction := "or"
	if e.allMustBeSpecified {
		conjunction = "and"
	}
	return fmt.Sprintf(`%s if %s %s specified`, errMsg, english.WordSeries(quoteStringSlice(e.conditionalFields), conjunction),
		english.PluralWord(len(e.conditionalFields), "is", "are"))
}

type errInvalidAutoscalingFieldsWithWkldType struct {
	invalidFields []string
	workloadType  string
}

func (e *errInvalidAutoscalingFieldsWithWkldType) Error() string {
	return fmt.Sprintf("autoscaling %v %v %v invalid with workload type %v", english.PluralWord(len(e.invalidFields), "field", "fields"),
		english.WordSeries(template.QuoteSliceFunc(e.invalidFields), "and"), english.PluralWord(len(e.invalidFields), "is", "are"), e.workloadType)
}

type errFieldMutualExclusive struct {
	firstField  string
	secondField string
	mustExist   bool
}

func (e *errFieldMutualExclusive) Error() string {
	if e.mustExist {
		return fmt.Sprintf(`must specify one of "%s" and "%s"`, e.firstField, e.secondField)
	}
	return fmt.Sprintf(`must specify one, not both, of "%s" and "%s"`, e.firstField, e.secondField)
}

type errGracePeriodsInBothALBAndNLB struct {
	errFieldMutualExclusive
}

func (e *errGracePeriodsInBothALBAndNLB) RecommendedActions() string {
	return `"grace_period" is a configuration shared by "http" and "nlb". Specify it only once under either "http" or "nlb".`
}

type errGracePeriodSpecifiedInAdditionalListener struct {
	index int
}

func (e *errGracePeriodSpecifiedInAdditionalListener) Error() string {
	return fmt.Sprintf(`"grace_period" specified for "nlb.additional_listeners[%d]"`, e.index)
}

// RecommendActions returns recommended actions to be taken after the error.
func (e *errGracePeriodSpecifiedInAdditionalListener) RecommendActions() string {
	return fmt.Sprintf(`Instead of under "nlb.additional_listeners[%d].healthcheck", specify "grace_period" under the top-level "nlb.healthcheck".`, e.index)

}

type errGracePeriodSpecifiedInAdditionalRule struct {
	index int
}

func (e *errGracePeriodSpecifiedInAdditionalRule) Error() string {
	return fmt.Sprintf(`"grace_period" specified for "http.additional_rules[%d]"`, e.index)
}

// RecommendActions returns recommended actions to be taken after the error.
func (e *errGracePeriodSpecifiedInAdditionalRule) RecommendActions() string {
	return fmt.Sprintf(`Instead of under "http.additional_rules[%d].healthcheck", specify "grace_period" under the top-level "http.healthcheck".`, e.index)

}

type errSpecifiedBothIngressFields struct {
	firstField  string
	secondField string
}

func (e *errSpecifiedBothIngressFields) Error() string {
	return fmt.Sprintf(`must specify one, not both, of "%s" and "%s"`, e.firstField, e.secondField)
}

// RecommendActions returns recommended actions to be taken after the error.
func (e *errSpecifiedBothIngressFields) RecommendActions() string {
	privateOrPublicField := strings.Split(e.firstField, ".")[0]
	if privateOrPublicField == "public" {
		return `
It looks like you specified ingress under both "http.public.security_groups.ingress" and "http.public.ingress".
After Copilot v1.23.0, we have deprecated "http.public.security_groups.ingress" in favor of "http.public.ingress". 
This means that "http.public.security_groups.ingress.cdn" is removed in favor of "http.public.ingress.cdn".
With the new release manifest configuration for cdn looks like:

http:
  public:
    ingress:
      cdn: true
`
	}

	return `
It looks like you specified ingress under both "http.private.security_groups.ingress" and "http.private.ingress".
After Copilot v1.23.0, we have deprecated "http.private.security_groups.ingress" in favor of "http.private.ingress". 
This means that "http.private.security_groups.ingress.from_vpc" is removed in favor of "http.private.ingress.vpc".
With the new release manifest configuration for vpc looks like:

http:
  private:
    ingress:
      vpc: true
`
}

type errRangeValueLessThanZero struct {
	min      int
	max      int
	spotFrom int
}

func (e *errRangeValueLessThanZero) Error() string {
	return fmt.Sprintf("min value %d, max value %d, and spot_from value %d must all be positive", e.min, e.max, e.spotFrom)
}

type errMinGreaterThanMax struct {
	min int
	max int
}

func (e *errMinGreaterThanMax) Error() string {
	return fmt.Sprintf("min value %d cannot be greater than max value %d", e.min, e.max)
}

type errAtLeastOneFieldMustBeSpecified struct {
	missingFields    []string
	conditionalField string
}

func (e *errAtLeastOneFieldMustBeSpecified) Error() string {
	errMsg := fmt.Sprintf("must specify at least one of %s", english.WordSeries(quoteStringSlice(e.missingFields), "or"))
	if e.conditionalField != "" {
		errMsg = fmt.Sprintf(`%s if "%s" is specified`, errMsg, e.conditionalField)
	}
	return errMsg
}

type errInvalidCloudFrontRegion struct{}

func (e *errInvalidCloudFrontRegion) Error() string {
	return fmt.Sprintf(`cdn certificate must be in region %s`, cloudfront.CertRegion)
}

// RecommendActions returns recommended actions to be taken after the error.
func (e *errInvalidCloudFrontRegion) RecommendActions() string {
	return fmt.Sprintf(`It looks like your CloudFront certificate is in the wrong region. CloudFront only supports certificates in %s.
We recommend creating a duplicate certificate in the %s region through AWS Certificate Manager.
More information: https://go.aws/3BMxY4J`, cloudfront.CertRegion, cloudfront.CertRegion)
}

func quoteStringSlice(in []string) []string {
	quoted := make([]string, len(in))
	for idx, str := range in {
		quoted[idx] = strconv.Quote(str)
	}
	return quoted
}

type errContainersExposingSamePort struct {
	firstContainer  string
	secondContainer string
	port            uint16
}

func (e *errContainersExposingSamePort) Error() string {
	return fmt.Sprintf(`containers %q and %q are exposing the same port %d`, e.firstContainer, e.secondContainer, e.port)
}

type errContainerPortExposedWithMultipleProtocol struct {
	container      string
	port           uint16
	firstProtocol  string
	secondProtocol string
}

func (e *errContainerPortExposedWithMultipleProtocol) Error() string {
	return fmt.Sprintf(`container %q is exposing the same port %d with protocol %s and %s`, e.container, e.port, e.firstProtocol, e.secondProtocol)
}

type errHealthCheckPortExposedWithInvalidProtocol struct {
	healthCheckPort uint16
	container       string
	protocol        string
}

func (e *errHealthCheckPortExposedWithInvalidProtocol) Error() string {
	return fmt.Sprintf(`container %q exposes port %d using protocol %s invalid for health checks. Valid protocol %s %s.`, 
		e.container, e.healthCheckPort, e.protocol, english.PluralWord(len(validHealthCheckProtocols), "is", "are"), 
		english.WordSeries(quoteStringSlice(validHealthCheckProtocols), "or"))
}
