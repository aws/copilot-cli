// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

// Range contains either a Range or a range configuration for Autoscaling ranges.
type Range struct {
	Value       *IntRangeBand // Mutually exclusive with RangeConfig
	RangeConfig RangeConfig
}

// ExposedPortsIndex holds exposed ports configuration.
type ExposedPortsIndex struct {
	WorkloadName      string                   // holds name of the main container
	PortsForContainer map[string][]ExposedPort // holds exposed ports list for all the containers
	ContainerForPort  map[uint16]string        // holds port to container mapping
}

func (idx ExposedPortsIndex) mainContainerPort() string {
	return idx.containerPortDefinedBy(idx.WorkloadName)
}

func (idx ExposedPortsIndex) mainContainerProtocol() string {
	return idx.containerProtocolDefinedBy(idx.WorkloadName)
}

// containerPortDefinedBy returns the explicitly defined container port, if there is no port exposed for the container then returns the empty string "".
func (idx ExposedPortsIndex) containerPortDefinedBy(container string) string {
	for _, portConfig := range idx.PortsForContainer[container] {
		if portConfig.isDefinedByContainer {
			return strconv.Itoa(int(portConfig.Port))
		}
	}
	return ""
}

// containerProtocolDefinedBy returns the protocol for the explicitly defined container port, if there is no port exposed for the container then returns the empty string "".
func (idx ExposedPortsIndex) containerProtocolDefinedBy(container string) string {
	for _, portConfig := range idx.PortsForContainer[container] {
		if portConfig.isDefinedByContainer {
			return portConfig.Protocol
		}
	}
	return ""
}

// IsEmpty returns whether Range is empty.
func (r *Range) IsEmpty() bool {
	return r.Value == nil && r.RangeConfig.IsEmpty()
}

// Parse extracts the min and max from RangeOpts.
func (r *Range) Parse() (min int, max int, err error) {
	if r.Value != nil {
		return r.Value.Parse()
	}
	return aws.IntValue(r.RangeConfig.Min), aws.IntValue(r.RangeConfig.Max), nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the RangeOpts
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (r *Range) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&r.RangeConfig); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !r.RangeConfig.IsEmpty() {
		// Unmarshaled successfully to r.RangeConfig, unset r.Range, and return.
		r.Value = nil
		return nil
	}

	if err := value.Decode(&r.Value); err != nil {
		return errUnmarshalRangeOpts
	}
	return nil
}

// IntRangeBand is a number range with maximum and minimum values.
type IntRangeBand string

// Parse parses Range string and returns the min and max values.
// For example: 1-100 returns 1 and 100.
func (r IntRangeBand) Parse() (min int, max int, err error) {
	minMax := strings.Split(string(r), "-")
	if len(minMax) != 2 {
		return 0, 0, fmt.Errorf("invalid range value %s. Should be in format of ${min}-${max}", string(r))
	}
	min, err = strconv.Atoi(minMax[0])
	if err != nil {
		return 0, 0, fmt.Errorf("cannot convert minimum value %s to integer", minMax[0])
	}
	max, err = strconv.Atoi(minMax[1])
	if err != nil {
		return 0, 0, fmt.Errorf("cannot convert maximum value %s to integer", minMax[1])
	}
	return min, max, nil
}

// RangeConfig containers a Min/Max and an optional SpotFrom field which
// specifies the number of services you want to start placing on spot. For
// example, if your range is 1-10 and `spot_from` is 5, up to 4 services will
// be placed on dedicated Fargate capacity, and then after that, any scaling
// event will place additioanl services on spot capacity.
type RangeConfig struct {
	Min      *int `yaml:"min"`
	Max      *int `yaml:"max"`
	SpotFrom *int `yaml:"spot_from"`
}

// IsEmpty returns whether RangeConfig is empty.
func (r *RangeConfig) IsEmpty() bool {
	return r.Min == nil && r.Max == nil && r.SpotFrom == nil
}

// Count is a custom type which supports unmarshaling yaml which
// can either be of type int or type AdvancedCount.
type Count struct {
	Value         *int          // 0 is a valid value, so we want the default value to be nil.
	AdvancedCount AdvancedCount // Mutually exclusive with Value.
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Count
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (c *Count) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&c.AdvancedCount); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !c.AdvancedCount.IsEmpty() {
		// Successfully unmarshalled AdvancedCount fields, return
		return nil
	}

	if err := value.Decode(&c.Value); err != nil {
		return errUnmarshalCountOpts
	}
	return nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the ScalingConfigOrT
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (r *ScalingConfigOrT[_]) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&r.ScalingConfig); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !r.ScalingConfig.IsEmpty() {
		// Successfully unmarshalled ScalingConfig fields, return
		return nil
	}

	if err := value.Decode(&r.Value); err != nil {
		return errors.New(`unable to unmarshal into int or composite-style map`)
	}
	return nil
}

// IsEmpty returns whether Count is empty.
func (c *Count) IsEmpty() bool {
	return c.Value == nil && c.AdvancedCount.IsEmpty()
}

// Desired returns the desiredCount to be set on the CFN template
func (c *Count) Desired() (*int, error) {
	if c.AdvancedCount.IsEmpty() {
		return c.Value, nil
	}

	if c.AdvancedCount.IgnoreRange() {
		return c.AdvancedCount.Spot, nil
	}
	min, _, err := c.AdvancedCount.Range.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse task count value %s: %w", aws.StringValue((*string)(c.AdvancedCount.Range.Value)), err)
	}
	return aws.Int(min), nil
}

// Percentage represents a valid percentage integer ranging from 0 to 100.
type Percentage int

// ScalingConfigOrT represents a resource that has autoscaling configurations or a generic value.
type ScalingConfigOrT[T ~int | time.Duration] struct {
	Value         *T
	ScalingConfig AdvancedScalingConfig[T] // mutually exclusive with Value
}

// AdvancedScalingConfig represents advanced configurable options for a scaling policy.
type AdvancedScalingConfig[T ~int | time.Duration] struct {
	Value    *T       `yaml:"value"`
	Cooldown Cooldown `yaml:"cooldown"`
}

// Cooldown represents the autoscaling cooldown of resources.
type Cooldown struct {
	ScaleInCooldown  *time.Duration `yaml:"in"`
	ScaleOutCooldown *time.Duration `yaml:"out"`
}

// AdvancedCount represents the configurable options for Auto Scaling as well as
// Capacity configuration (spot).
type AdvancedCount struct {
	Spot         *int                            `yaml:"spot"` // mutually exclusive with other fields
	Range        Range                           `yaml:"range"`
	Cooldown     Cooldown                        `yaml:"cooldown"`
	CPU          ScalingConfigOrT[Percentage]    `yaml:"cpu_percentage"`
	Memory       ScalingConfigOrT[Percentage]    `yaml:"memory_percentage"`
	Requests     ScalingConfigOrT[int]           `yaml:"requests"`
	ResponseTime ScalingConfigOrT[time.Duration] `yaml:"response_time"`
	QueueScaling QueueScaling                    `yaml:"queue_delay"`

	workloadType string
}

// IsEmpty returns whether ScalingConfigOrT is empty
func (r *ScalingConfigOrT[_]) IsEmpty() bool {
	return r.ScalingConfig.IsEmpty() && r.Value == nil
}

// IsEmpty returns whether AdvancedScalingConfig is empty
func (a *AdvancedScalingConfig[_]) IsEmpty() bool {
	return a.Cooldown.IsEmpty() && a.Value == nil
}

// IsEmpty returns whether Cooldown is empty
func (c *Cooldown) IsEmpty() bool {
	return c.ScaleInCooldown == nil && c.ScaleOutCooldown == nil
}

// IsEmpty returns whether AdvancedCount is empty.
func (a *AdvancedCount) IsEmpty() bool {
	return a.Range.IsEmpty() && a.CPU.IsEmpty() && a.Memory.IsEmpty() && a.Cooldown.IsEmpty() &&
		a.Requests.IsEmpty() && a.ResponseTime.IsEmpty() && a.Spot == nil && a.QueueScaling.IsEmpty()
}

// IgnoreRange returns whether desiredCount is specified on spot capacity
func (a *AdvancedCount) IgnoreRange() bool {
	return a.Spot != nil
}

func (a *AdvancedCount) hasAutoscaling() bool {
	return !a.Range.IsEmpty() || a.hasScalingFieldsSet()
}

func (a *AdvancedCount) validScalingFields() []string {
	switch a.workloadType {
	case manifestinfo.LoadBalancedWebServiceType:
		return []string{"cpu_percentage", "memory_percentage", "requests", "response_time"}
	case manifestinfo.BackendServiceType:
		return []string{"cpu_percentage", "memory_percentage", "requests", "response_time"}
	case manifestinfo.WorkerServiceType:
		return []string{"cpu_percentage", "memory_percentage", "queue_delay"}
	default:
		return nil
	}
}

func (a *AdvancedCount) hasScalingFieldsSet() bool {
	switch a.workloadType {
	case manifestinfo.LoadBalancedWebServiceType:
		return !a.CPU.IsEmpty() || !a.Memory.IsEmpty() || !a.Requests.IsEmpty() || !a.ResponseTime.IsEmpty()
	case manifestinfo.BackendServiceType:
		return !a.CPU.IsEmpty() || !a.Memory.IsEmpty() || !a.Requests.IsEmpty() || !a.ResponseTime.IsEmpty()
	case manifestinfo.WorkerServiceType:
		return !a.CPU.IsEmpty() || !a.Memory.IsEmpty() || !a.QueueScaling.IsEmpty()
	default:
		return !a.CPU.IsEmpty() || !a.Memory.IsEmpty() || !a.Requests.IsEmpty() || !a.ResponseTime.IsEmpty() || !a.QueueScaling.IsEmpty()
	}
}

func (a *AdvancedCount) getInvalidFieldsSet() []string {
	var invalidFields []string

	switch a.workloadType {
	case manifestinfo.LoadBalancedWebServiceType:
		if !a.QueueScaling.IsEmpty() {
			invalidFields = append(invalidFields, "queue_delay")
		}
	case manifestinfo.BackendServiceType:
		if !a.QueueScaling.IsEmpty() {
			invalidFields = append(invalidFields, "queue_delay")
		}
	case manifestinfo.WorkerServiceType:
		if !a.Requests.IsEmpty() {
			invalidFields = append(invalidFields, "requests")
		}
		if !a.ResponseTime.IsEmpty() {
			invalidFields = append(invalidFields, "response_time")
		}
	}
	return invalidFields
}

func (a *AdvancedCount) unsetAutoscaling() {
	a.Range = Range{}
	a.Cooldown = Cooldown{}
	a.CPU = ScalingConfigOrT[Percentage]{}
	a.Memory = ScalingConfigOrT[Percentage]{}
	a.Requests = ScalingConfigOrT[int]{}
	a.ResponseTime = ScalingConfigOrT[time.Duration]{}
	a.QueueScaling = QueueScaling{}
}

// QueueScaling represents the configuration to scale a service based on a SQS queue.
type QueueScaling struct {
	AcceptableLatency *time.Duration `yaml:"acceptable_latency"`
	AvgProcessingTime *time.Duration `yaml:"msg_processing_time"`
	Cooldown          Cooldown       `yaml:"cooldown"`
}

// IsEmpty returns true if the QueueScaling is set.
func (qs *QueueScaling) IsEmpty() bool {
	return qs.AcceptableLatency == nil && qs.AvgProcessingTime == nil && qs.Cooldown.IsEmpty()
}

// AcceptableBacklogPerTask returns the total number of messages that each task can accumulate in the queue
// while maintaining the AcceptableLatency given the AvgProcessingTime.
func (qs *QueueScaling) AcceptableBacklogPerTask() (int, error) {
	if qs.IsEmpty() {
		return 0, errors.New(`"queue_delay" must be specified in order to calculate the acceptable backlog`)
	}
	v := math.Ceil(float64(*qs.AcceptableLatency) / float64(*qs.AvgProcessingTime))
	return int(v), nil
}

// HTTPHealthCheckArgs holds the configuration to determine if the load balanced web service is healthy.
// These options are specifiable under the "healthcheck" field.
// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html.
type HTTPHealthCheckArgs struct {
	Path               *string        `yaml:"path"`
	Port               *int           `yaml:"port"`
	SuccessCodes       *string        `yaml:"success_codes"`
	HealthyThreshold   *int64         `yaml:"healthy_threshold"`
	UnhealthyThreshold *int64         `yaml:"unhealthy_threshold"`
	Timeout            *time.Duration `yaml:"timeout"`
	Interval           *time.Duration `yaml:"interval"`
	GracePeriod        *time.Duration `yaml:"grace_period"`
}

// HealthCheckArgsOrString is a custom type which supports unmarshaling yaml which
// can either be of type string or type HealthCheckArgs.
type HealthCheckArgsOrString struct {
	Union[string, HTTPHealthCheckArgs]
}

// Path returns the default health check path if provided otherwise, returns the path from the advanced configuration.
func (hc *HealthCheckArgsOrString) Path() *string {
	if hc.IsBasic() {
		return aws.String(hc.Basic)
	}
	return hc.Advanced.Path
}

// NLBHealthCheckArgs holds the configuration to determine if the network load balanced web service is healthy.
// These options are specifiable under the "healthcheck" field.
// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html.
type NLBHealthCheckArgs struct {
	Port               *int           `yaml:"port"`
	HealthyThreshold   *int64         `yaml:"healthy_threshold"`
	UnhealthyThreshold *int64         `yaml:"unhealthy_threshold"`
	Timeout            *time.Duration `yaml:"timeout"`
	Interval           *time.Duration `yaml:"interval"`
	GracePeriod        *time.Duration `yaml:"grace_period"`
}

func (h *NLBHealthCheckArgs) isEmpty() bool {
	return h.Port == nil && h.HealthyThreshold == nil && h.UnhealthyThreshold == nil && h.Timeout == nil && h.Interval == nil
}

// ParsePortMapping parses port-protocol string into individual port and protocol strings.
// Valid examples: 2000/udp, or 2000.
func ParsePortMapping(s *string) (port *string, protocol *string, err error) {
	if s == nil {
		return nil, nil, nil
	}
	portProtocol := strings.Split(*s, "/")
	switch len(portProtocol) {
	case 1:
		return aws.String(portProtocol[0]), nil, nil
	case 2:
		return aws.String(portProtocol[0]), aws.String(portProtocol[1]), nil
	default:
		return nil, nil, fmt.Errorf("cannot parse port mapping from %s", *s)
	}
}

type fromCFN struct {
	Name *string `yaml:"from_cfn"`
}

func (cfg *fromCFN) isEmpty() bool {
	return cfg.Name == nil
}

// StringOrFromCFN represents a choice between a plain string value and a value retrieved from CloudFormation.
type StringOrFromCFN struct {
	Plain   *string // Plain is a user-defined string value.
	FromCFN fromCFN // FromCFN holds a value obtained from CloudFormation.
}

func (s StringOrFromCFN) isEmpty() bool {
	return s.FromCFN.isEmpty() && s.Plain == nil
}

// UnmarshalYAML implements the yaml.Unmarshaler (v3) interface to override the default YAML unmarshalling logic.
func (s *StringOrFromCFN) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&s.FromCFN); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}
	if !s.FromCFN.isEmpty() { // Successfully unmarshalled to a environment import name.
		return nil
	}
	if err := value.Decode(&s.Plain); err != nil { // Otherwise, try decoding the simple form.
		return errors.New(`cannot unmarshal field to a string or into a map`)
	}
	return nil
}

func (cfg ImageWithPortAndHealthcheck) exposePorts(exposedPorts map[uint16]ExposedPort, workloadName string) map[uint16]ExposedPort {
	if cfg.Port == nil {
		return nil
	}

	newExposedPorts := make(map[uint16]ExposedPort)
	targetPort := aws.Uint16Value(cfg.Port)
	if exposedPort, ok := exposedPorts[targetPort]; ok {
		newExposedPorts[targetPort] = ExposedPort{
			Port:                 exposedPort.Port,
			Protocol:             exposedPort.Protocol,
			ContainerName:        exposedPort.ContainerName,
			isDefinedByContainer: true,
		}
		return newExposedPorts
	}

	exposedPorts[targetPort] = ExposedPort{
		Port:                 targetPort,
		Protocol:             strings.ToLower(defaultProtocol),
		ContainerName:        workloadName,
		isDefinedByContainer: true,
	}
	return newExposedPorts
}

func (cfg ImageWithHealthcheckAndOptionalPort) exposePorts(exposedPorts map[uint16]ExposedPort, workloadName string) map[uint16]ExposedPort {
	if cfg.Port == nil {
		return nil
	}

	newExposedPorts := make(map[uint16]ExposedPort)
	targetPort := aws.Uint16Value(cfg.Port)
	if exposedPort, ok := exposedPorts[targetPort]; ok {
		newExposedPorts[targetPort] = ExposedPort{
			Port:                 exposedPort.Port,
			Protocol:             exposedPort.Protocol,
			ContainerName:        exposedPort.ContainerName,
			isDefinedByContainer: true,
		}
		return newExposedPorts
	}

	exposedPorts[targetPort] = ExposedPort{
		Port:                 targetPort,
		Protocol:             strings.ToLower(defaultProtocol),
		ContainerName:        workloadName,
		isDefinedByContainer: true,
	}
	return newExposedPorts
}

// exposePorts populates a map of ports that should be exposed given the application load balancer
// configuration that's not part of the existing containerPorts.
func (rule RoutingRule) exposePorts(exposedPorts map[uint16]ExposedPort, workloadName string) map[uint16]ExposedPort {
	if rule.TargetPort == nil {
		return nil
	}
	targetContainer := workloadName
	if rule.TargetContainer != nil {
		targetContainer = aws.StringValue(rule.TargetContainer)
	}
	targetPort := aws.Uint16Value(rule.TargetPort)
	if _, ok := exposedPorts[targetPort]; ok {
		return nil
	}
	newExposedPorts := make(map[uint16]ExposedPort)
	newExposedPorts[targetPort] = ExposedPort{
		Port:          targetPort,
		Protocol:      strings.ToLower(TCP),
		ContainerName: targetContainer,
	}
	return newExposedPorts
}

// exposePorts populates a map of ports that should be exposed given the network load balancer
// configuration that's not part of the existing containerPorts.
func (cfg NetworkLoadBalancerListener) exposePorts(exposedPorts map[uint16]ExposedPort, workloadName string) (map[uint16]ExposedPort, error) {
	if cfg.IsEmpty() {
		return nil, nil
	}
	nlbPort, nlbProtocol, err := ParsePortMapping(cfg.Port)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseUint(aws.StringValue(nlbPort), 10, 16)
	if err != nil {
		return nil, err
	}
	targetPort := uint16(port)
	if cfg.TargetPort != nil {
		targetPort = uint16(aws.IntValue(cfg.TargetPort))
	}
	targetProtocol := TCP
	if nlbProtocol != nil {
		// Expose TCP port for TLS listeners.
		if protocol := aws.StringValue(nlbProtocol); !strings.EqualFold(protocol, TLS) {
			targetProtocol = protocol
		}
	}
	targetProtocol = strings.ToLower(targetProtocol)
	for _, exposedPort := range exposedPorts {
		if targetPort == exposedPort.Port && targetProtocol == exposedPort.Protocol {
			return nil, nil
		}
	}
	targetContainer := workloadName
	if cfg.TargetContainer != nil {
		targetContainer = aws.StringValue(cfg.TargetContainer)
	}

	newExposedPorts := make(map[uint16]ExposedPort)
	newExposedPorts[targetPort] = ExposedPort{
		Port:          targetPort,
		Protocol:      targetProtocol,
		ContainerName: targetContainer,
	}

	return newExposedPorts, nil
}

func (sidecar SidecarConfig) exposePorts(exposedPorts map[uint16]ExposedPort, sidecarName string) (map[uint16]ExposedPort, error) {
	if sidecar.Port == nil {
		return nil, nil
	}
	sidecarPort, protocolPtr, err := ParsePortMapping(sidecar.Port)
	if err != nil {
		return nil, err
	}
	protocol := aws.StringValue(protocolPtr)
	if protocolPtr == nil {
		protocol = "tcp"
	}
	port, err := strconv.ParseUint(aws.StringValue(sidecarPort), 10, 16)
	if err != nil {
		return nil, err
	}

	newExposedPorts := make(map[uint16]ExposedPort)
	newExposedPorts[uint16(port)] = ExposedPort{
		Port:                 uint16(port),
		Protocol:             strings.ToLower(protocol),
		ContainerName:        sidecarName,
		isDefinedByContainer: true,
	}

	return newExposedPorts, nil
}

// ServiceConnectTargetContainer contains the name of a container and port to expose to ECS Service Connect.
type ServiceConnectTargetContainer struct {
	Container string
	Port      string
}

// ServiceConnectTarget returns the target container, port, and protocol to be exposed for ServiceConnect.
func (l *LoadBalancedWebService) ServiceConnectTarget(exposedPorts ExposedPortsIndex) *ServiceConnectTargetContainer {
	// Expose ServiceConnect from `image.port` by default.
	targetContainer := exposedPorts.WorkloadName
	targetPort := exposedPorts.mainContainerPort()
	targetProtocol := exposedPorts.mainContainerProtocol()

	// Only assign albContainer and albPort if alb is enabled.
	var albContainer, albPort *string
	if !l.HTTPOrBool.Disabled() {
		albContainer, albPort = l.HTTPOrBool.Main.exposedContainerAndPort(exposedPorts)
	}
	if albContainer != nil && albPort != nil {
		targetContainer = aws.StringValue(albContainer)
		targetPort = aws.StringValue(albPort)
		targetProtocol = TCP
	}

	// ServiceConnect can't use UDP.
	if strings.EqualFold(targetProtocol, UDP) {
		return nil
	}

	return &ServiceConnectTargetContainer{
		Container: targetContainer,
		Port:      targetPort,
	}
}

// ServiceConnectTarget returns the target container and port to be exposed for ServiceConnect.
func (b *BackendService) ServiceConnectTarget(exposedPorts ExposedPortsIndex) *ServiceConnectTargetContainer {
	// Expose ServiceConnect from `image.port` by default.
	targetContainer := exposedPorts.WorkloadName
	targetPort := exposedPorts.mainContainerPort()
	targetProtocol := exposedPorts.mainContainerProtocol()

	albContainer, albPort := b.HTTP.Main.exposedContainerAndPort(exposedPorts)
	if albContainer != nil && albPort != nil {
		targetContainer = aws.StringValue(albContainer)
		targetPort = aws.StringValue(albPort)
		targetProtocol = TCP
	}

	// ServiceConnect can't use UDP.
	if strings.EqualFold(targetProtocol, UDP) {
		return nil
	}

	return &ServiceConnectTargetContainer{
		Container: targetContainer,
		Port:      targetPort,
	}
}

// Target returns target container and target port for the ALB configuration.
// This method should be called only when ALB config is not empty.
func (rule *RoutingRule) Target(exposedPorts ExposedPortsIndex) (targetContainer string, targetPort string, err error) {
	// Route load balancer traffic to main container by default.
	targetContainer = exposedPorts.WorkloadName
	targetPort = exposedPorts.mainContainerPort()

	ruleTargetContainer, ruleTargetPort := rule.exposedContainerAndPort(exposedPorts)
	if ruleTargetContainer != nil {
		targetContainer = aws.StringValue(ruleTargetContainer)
	}
	if ruleTargetPort != nil {
		targetPort = aws.StringValue(ruleTargetPort)
	}
	return
}

// exposedContainerAndPort returns the targetContainer and targetPort from a given ExposedPortsIndex for a routing rule.
func (rule *RoutingRule) exposedContainerAndPort(exposedPorts ExposedPortsIndex) (*string, *string) {
	var targetContainer, targetPort *string

	if rule.TargetContainer == nil && rule.TargetPort == nil { // both targetPort and targetContainer are nil.
		return nil, nil
	}

	if rule.TargetPort == nil { // when target_port is nil
		if aws.StringValue(rule.TargetContainer) != exposedPorts.WorkloadName {
			targetContainer = rule.TargetContainer
			targetPort = aws.String(exposedPorts.containerPortDefinedBy(aws.StringValue(rule.TargetContainer)))
			/* NOTE: When the `target_port` is empty, the intended target port should be the port that is explicitly exposed by the container. Consider the following example
			```
			http:
			  target_container: nginx
			sidecars:
			  nginx:
			    port: 81 # Explicitly exposed by the nginx container.
			```
			In this example, the target port for the ALB listener rule should be 81
			*/
		}
		return targetContainer, targetPort
	}

	if rule.TargetContainer == nil { // when target_container is nil
		container, port := targetContainerFromTargetPort(exposedPorts, rule.TargetPort)
		targetPort = port
		// In general, containers aren't expected to be empty. But this condition is applied for extra safety.
		if container != nil {
			targetContainer = container
		}
		return targetContainer, targetPort
	}

	// when both target_port and target_container are not nil
	targetContainer = rule.TargetContainer
	targetPort = aws.String(template.StrconvUint16(aws.Uint16Value(rule.TargetPort)))
	return targetContainer, targetPort
}

// targetContainerFromTargetPort returns target container and target port from the given target_port input.
func targetContainerFromTargetPort(exposedPorts ExposedPortsIndex, port *uint16) (targetContainer *string, targetPort *string) {
	// Route load balancer traffic to the target_port if mentioned.
	targetPort = aws.String(template.StrconvUint16(aws.Uint16Value(port)))
	// It shouldn’t be possible that container is empty for the given port as exposed port assigns container to all the ports, this is just for the extra safety.
	if exposedPorts.ContainerForPort[aws.Uint16Value(port)] != "" {
		targetContainer = aws.String(exposedPorts.ContainerForPort[aws.Uint16Value(port)])
	}
	return
}

// MainContainerPort returns the main container port.
func (s *LoadBalancedWebService) MainContainerPort() string {
	return strconv.FormatUint(uint64(aws.Uint16Value(s.ImageConfig.Port)), 10)
}

// MainContainerPort returns the main container port if given.
func (s *BackendService) MainContainerPort() string {
	port := template.NoExposedContainerPort
	if s.BackendServiceConfig.ImageConfig.Port != nil {
		port = strconv.FormatUint(uint64(aws.Uint16Value(s.BackendServiceConfig.ImageConfig.Port)), 10)
	}
	return port
}

func prepareParsedExposedPortsMap(exposedPorts map[uint16]ExposedPort) (map[string][]ExposedPort, map[uint16]string) {
	parsedContainerMap := make(map[string][]ExposedPort)
	parsedExposedPortMap := make(map[uint16]string)

	var ports []uint16
	for port := range exposedPorts {
		ports = append(ports, port)
	}
	// Sort for consistency in unit tests.
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })

	for _, port := range ports {
		exposedPort := exposedPorts[port]
		parsedContainerMap[exposedPort.ContainerName] = append(parsedContainerMap[exposedPort.ContainerName], exposedPort)
		parsedExposedPortMap[exposedPort.Port] = exposedPort.ContainerName
	}
	return parsedContainerMap, parsedExposedPortMap
}

// Target returns target container and target port for a NLB listener configuration.
func (listener NetworkLoadBalancerListener) Target(exposedPorts ExposedPortsIndex) (targetContainer string, targetPort string, err error) {
	// Parse listener port and protocol.
	port, _, err := ParsePortMapping(listener.Port)
	if err != nil {
		return "", "", err
	}
	// By default, the target port is the same as listener port.
	targetPort = aws.StringValue(port)
	targetContainer = exposedPorts.WorkloadName
	if listener.TargetContainer == nil && listener.TargetPort == nil { // both targetPort and targetContainer are nil.
		return
	}

	if listener.TargetPort == nil { // when target_port is nil
		if aws.StringValue(listener.TargetContainer) != exposedPorts.WorkloadName {
			targetContainer = aws.StringValue(listener.TargetContainer)
			for _, portConfig := range exposedPorts.PortsForContainer[targetContainer] {
				if portConfig.isDefinedByContainer {
					targetPort = strconv.Itoa(int(portConfig.Port))
					/* NOTE: When the `target_port` is empty, the intended target port should be the port that is explicitly exposed by the container. Consider the following example
					```
					http:
					  target_container: nginx
					  target_port: 83 # Implicitly exposed by the nginx container
					nlb:
					  port: 80/tcp
					  target_container: nginx
					sidecars:
					  nginx:
					    port: 81 # Explicitly exposed by the nginx container.
					```
					In this example, the target port for the NLB listener should be 81
					*/
				}
			}
		}
		return
	}

	if listener.TargetContainer == nil { // when target_container is nil
		container, port := targetContainerFromTargetPort(exposedPorts, uint16P(uint16(aws.IntValue(listener.TargetPort))))
		targetPort = aws.StringValue(port)
		// In general, containers aren't expected to be empty. But this condition is applied for extra safety.
		if container != nil {
			targetContainer = aws.StringValue(container)
		}
		return
	}

	// when both target_port and target_container are not nil
	targetContainer = aws.StringValue(listener.TargetContainer)
	targetPort = template.StrconvUint16(uint16(aws.IntValue(listener.TargetPort)))
	return
}
