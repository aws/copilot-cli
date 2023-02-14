// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"gopkg.in/yaml.v3"
)

// Environmentmanifestinfo identifies that the type of manifest is environment manifest.
const Environmentmanifestinfo = "Environment"

var environmentManifestPath = "environment/manifest.yml"

// Error definitions.
var (
	errUnmarshalPortsConfig          = errors.New(`unable to unmarshal ports field into int or a range`)
	errUnmarshalEnvironmentCDNConfig = errors.New(`unable to unmarshal cdn field into bool or composite-style map`)
	errUnmarshalELBAccessLogs        = errors.New(`unable to unmarshal access_logs field into bool or ELB Access logs config`)
)

// Environment is the manifest configuration for an environment.
type Environment struct {
	Workload          `yaml:",inline"`
	EnvironmentConfig `yaml:",inline"`

	parser template.Parser
}

// EnvironmentProps contains properties for creating a new environment manifest.
type EnvironmentProps struct {
	Name         string
	CustomConfig *config.CustomizeEnv
	Telemetry    *config.Telemetry
}

// NewEnvironment creates a new environment manifest object.
func NewEnvironment(props *EnvironmentProps) *Environment {
	return FromEnvConfig(&config.Environment{
		Name:         props.Name,
		CustomConfig: props.CustomConfig,
		Telemetry:    props.Telemetry,
	}, template.New())
}

// FromEnvConfig transforms an environment configuration into a manifest.
func FromEnvConfig(cfg *config.Environment, parser template.Parser) *Environment {
	var vpc environmentVPCConfig
	vpc.loadVPCConfig(cfg.CustomConfig)

	var http EnvironmentHTTPConfig
	http.loadLBConfig(cfg.CustomConfig)

	var obs environmentObservability
	obs.loadObsConfig(cfg.Telemetry)

	return &Environment{
		Workload: Workload{
			Name: stringP(cfg.Name),
			Type: stringP(Environmentmanifestinfo),
		},
		EnvironmentConfig: EnvironmentConfig{
			Network: environmentNetworkConfig{
				VPC: vpc,
			},
			HTTPConfig:    http,
			Observability: obs,
		},
		parser: parser,
	}
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (e *Environment) MarshalBinary() ([]byte, error) {
	content, err := e.parser.Parse(environmentManifestPath, *e, template.WithFuncs(map[string]interface{}{
		"fmtStringSlice": template.FmtSliceFunc,
	}))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// EnvironmentConfig defines the configuration settings for an environment manifest
type EnvironmentConfig struct {
	Network       environmentNetworkConfig `yaml:"network,omitempty,flow"`
	Observability environmentObservability `yaml:"observability,omitempty,flow"`
	HTTPConfig    EnvironmentHTTPConfig    `yaml:"http,omitempty,flow"`
	CDNConfig     EnvironmentCDNConfig     `yaml:"cdn,omitempty,flow"`
}

// IsPublicLBIngressRestrictedToCDN returns whether an environment has its
// Public Load Balancer ingress restricted to a Content Delivery Network.
func (mft *EnvironmentConfig) IsPublicLBIngressRestrictedToCDN() bool {
	// Check the fixed manifest first. This would be `http.public.ingress.cdn`.
	// For more information, see https://github.com/aws/copilot-cli/pull/4068#issuecomment-1275080333
	if !mft.HTTPConfig.Public.Ingress.IsEmpty() {
		return aws.BoolValue(mft.HTTPConfig.Public.Ingress.CDNIngress)
	}
	// Fall through to the old manifest: `http.public.security_groups.ingress.cdn`.
	return aws.BoolValue(mft.HTTPConfig.Public.DeprecatedSG.DeprecatedIngress.RestrictiveIngress.CDNIngress)
}

// GetPublicALBSourceIPs returns list of IPNet.
func (mft *EnvironmentConfig) GetPublicALBSourceIPs() []IPNet {
	return mft.HTTPConfig.Public.Ingress.SourceIPs
}

type environmentNetworkConfig struct {
	VPC environmentVPCConfig `yaml:"vpc,omitempty"`
}

type environmentVPCConfig struct {
	ID                  *string                       `yaml:"id,omitempty"`
	CIDR                *IPNet                        `yaml:"cidr,omitempty"`
	Subnets             subnetsConfiguration          `yaml:"subnets,omitempty"`
	SecurityGroupConfig securityGroupConfig           `yaml:"security_group,omitempty"`
	FlowLogs            Union[*bool, VPCFlowLogsArgs] `yaml:"flow_logs,omitempty"`
}

type securityGroupConfig struct {
	Ingress []securityGroupRule `yaml:"ingress,omitempty"`
	Egress  []securityGroupRule `yaml:"egress,omitempty"`
}

func (cfg securityGroupConfig) isEmpty() bool {
	return len(cfg.Ingress) == 0 && len(cfg.Egress) == 0
}

// securityGroupRule holds the security group ingress and egress configs.
type securityGroupRule struct {
	CidrIP     string      `yaml:"cidr"`
	Ports      portsConfig `yaml:"ports"`
	IpProtocol string      `yaml:"ip_protocol"`
}

// portsConfig represents a range of ports [from:to] inclusive.
// The simple form allow represents from and to ports as a single value, whereas the advanced form is for different values.
type portsConfig struct {
	Port  *int          // 0 is a valid value, so we want the default value to be nil.
	Range *IntRangeBand // Mutually exclusive with port.
}

// IsEmpty returns whether PortsConfig is empty.
func (cfg *portsConfig) IsEmpty() bool {
	return cfg.Port == nil && cfg.Range == nil
}

// GetPorts returns the from and to ports of a security group rule.
func (r securityGroupRule) GetPorts() (from, to int, err error) {
	if r.Ports.Range == nil {
		return aws.IntValue(r.Ports.Port), aws.IntValue(r.Ports.Port), nil // a single value is provided for ports.
	}
	return r.Ports.Range.Parse()
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Ports
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (cfg *portsConfig) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&cfg.Port); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			cfg.Port = nil
		default:
			return err
		}
	}

	if cfg.Port != nil {
		// Successfully unmarshalled Port field and unset Ports field, return
		cfg.Range = nil
		return nil
	}

	if err := value.Decode(&cfg.Range); err != nil {
		return errUnmarshalPortsConfig
	}
	return nil
}

// VPCFlowLogsArgs holds the flow logs configuration.
type VPCFlowLogsArgs struct {
	Retention *int `yaml:"retention,omitempty"`
}

// IsZero implements yaml.IsZeroer.
func (fl *VPCFlowLogsArgs) IsZero() bool {
	return fl.Retention == nil
}

// EnvSecurityGroup returns the security group config if the user has set any values.
// If there is no env security group settings, then returns nil and false.
func (cfg *EnvironmentConfig) EnvSecurityGroup() (*securityGroupConfig, bool) {
	if isEmpty := cfg.Network.VPC.SecurityGroupConfig.isEmpty(); !isEmpty {
		return &cfg.Network.VPC.SecurityGroupConfig, true
	}
	return nil, false
}

// EnvironmentCDNConfig represents configuration of a CDN.
type EnvironmentCDNConfig struct {
	Enabled *bool
	Config  AdvancedCDNConfig // mutually exclusive with Enabled
}

// AdvancedCDNConfig represents an advanced configuration for a Content Delivery Network.
type AdvancedCDNConfig struct {
	Certificate  *string         `yaml:"certificate,omitempty"`
	TerminateTLS *bool           `yaml:"terminate_tls,omitempty"`
	Static       CDNStaticConfig `yaml:"static_assets,omitempty"`
}

// IsEmpty returns whether environmentCDNConfig is empty.
func (cfg *EnvironmentCDNConfig) IsEmpty() bool {
	return cfg.Enabled == nil && cfg.Config.isEmpty()
}

// isEmpty returns whether advancedCDNConfig is empty.
func (cfg *AdvancedCDNConfig) isEmpty() bool {
	return cfg.Certificate == nil && cfg.TerminateTLS == nil && cfg.Static.IsEmpty()
}

// CDNEnabled returns whether a CDN configuration has been enabled in the environment manifest.
func (cfg *EnvironmentConfig) CDNEnabled() bool {
	if !cfg.CDNConfig.Config.isEmpty() {
		return true
	}
	return aws.BoolValue(cfg.CDNConfig.Enabled)
}

// HasImportedPublicALBCerts returns true when the environment's ALB
// is configured with certs for the public listener.
func (cfg *EnvironmentConfig) HasImportedPublicALBCerts() bool {
	return cfg.HTTPConfig.Public.Certificates != nil
}

// CDNDoesTLSTermination returns true when the environment's CDN
// is configured to terminate incoming TLS connections.
func (cfg *EnvironmentConfig) CDNDoesTLSTermination() bool {
	return aws.BoolValue(cfg.CDNConfig.Config.TerminateTLS)
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the environmentCDNConfig
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (cfg *EnvironmentCDNConfig) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&cfg.Config); err != nil {
		var yamlTypeErr *yaml.TypeError
		if !errors.As(err, &yamlTypeErr) {
			return err
		}
	}

	if !cfg.Config.isEmpty() {
		// Successfully unmarshalled CDNConfig fields, return
		return nil
	}

	if err := value.Decode(&cfg.Enabled); err != nil {
		return errUnmarshalEnvironmentCDNConfig
	}
	return nil
}

// CDNStaticConfig represents the static config for CDN.
type CDNStaticConfig struct {
	Location string `yaml:"location,omitempty"`
	Alias    string `yaml:"alias,omitempty"`
	Path     string `yaml:"path,omitempty"`
}

// IsEmpty returns true if CDNStaticConfig is not configured.
func (cfg CDNStaticConfig) IsEmpty() bool {
	return cfg.Location == "" && cfg.Alias == "" && cfg.Path == ""
}

// IsEmpty returns true if environmentVPCConfig is not configured.
func (cfg environmentVPCConfig) IsEmpty() bool {
	return cfg.ID == nil && cfg.CIDR == nil && cfg.Subnets.IsEmpty() && cfg.FlowLogs.IsZero()
}

func (cfg *environmentVPCConfig) loadVPCConfig(env *config.CustomizeEnv) {
	if env.IsEmpty() {
		return
	}
	if adjusted := env.VPCConfig; adjusted != nil {
		cfg.loadAdjustedVPCConfig(adjusted)
	}
	if imported := env.ImportVPC; imported != nil {
		cfg.loadImportedVPCConfig(imported)
	}
}

func (cfg *environmentVPCConfig) loadAdjustedVPCConfig(vpc *config.AdjustVPC) {
	cfg.CIDR = ipNetP(vpc.CIDR)
	cfg.Subnets.Public = make([]subnetConfiguration, len(vpc.PublicSubnetCIDRs))
	cfg.Subnets.Private = make([]subnetConfiguration, len(vpc.PrivateSubnetCIDRs))
	for i, cidr := range vpc.PublicSubnetCIDRs {
		cfg.Subnets.Public[i].CIDR = ipNetP(cidr)
		if len(vpc.AZs) > i {
			cfg.Subnets.Public[i].AZ = stringP(vpc.AZs[i])
		}
	}
	for i, cidr := range vpc.PrivateSubnetCIDRs {
		cfg.Subnets.Private[i].CIDR = ipNetP(cidr)
		if len(vpc.AZs) > i {
			cfg.Subnets.Private[i].AZ = stringP(vpc.AZs[i])
		}
	}
}

func (cfg *environmentVPCConfig) loadImportedVPCConfig(vpc *config.ImportVPC) {
	cfg.ID = stringP(vpc.ID)
	cfg.Subnets.Public = make([]subnetConfiguration, len(vpc.PublicSubnetIDs))
	for i, subnet := range vpc.PublicSubnetIDs {
		cfg.Subnets.Public[i].SubnetID = stringP(subnet)
	}
	cfg.Subnets.Private = make([]subnetConfiguration, len(vpc.PrivateSubnetIDs))
	for i, subnet := range vpc.PrivateSubnetIDs {
		cfg.Subnets.Private[i].SubnetID = stringP(subnet)
	}
}

// UnmarshalEnvironment deserializes the YAML input stream into an environment manifest object.
// If an error occurs during deserialization, then returns the error.
func UnmarshalEnvironment(in []byte) (*Environment, error) {
	var m Environment
	if err := yaml.Unmarshal(in, &m); err != nil {
		return nil, fmt.Errorf("unmarshal environment manifest: %w", err)
	}
	return &m, nil
}

func (cfg *environmentVPCConfig) imported() bool {
	return aws.StringValue(cfg.ID) != ""
}

func (cfg *environmentVPCConfig) managedVPCCustomized() bool {
	return aws.StringValue((*string)(cfg.CIDR)) != ""
}

// ImportedVPC returns configurations that import VPC resources if there is any.
func (cfg *environmentVPCConfig) ImportedVPC() *template.ImportVPC {
	if !cfg.imported() {
		return nil
	}
	var publicSubnetIDs, privateSubnetIDs []string
	for _, subnet := range cfg.Subnets.Public {
		publicSubnetIDs = append(publicSubnetIDs, aws.StringValue(subnet.SubnetID))
	}
	for _, subnet := range cfg.Subnets.Private {
		privateSubnetIDs = append(privateSubnetIDs, aws.StringValue(subnet.SubnetID))
	}
	return &template.ImportVPC{
		ID:               aws.StringValue(cfg.ID),
		PublicSubnetIDs:  publicSubnetIDs,
		PrivateSubnetIDs: privateSubnetIDs,
	}
}

// ManagedVPC returns configurations that configure VPC resources if there is any.
func (cfg *environmentVPCConfig) ManagedVPC() *template.ManagedVPC {
	// ASSUMPTION: If the VPC is configured, both pub and private are explicitly configured.
	// az is optional. However, if it's configured, it is configured for all subnets.
	// In summary:
	// 0 = #pub = #priv = #azs (not managed)
	// #pub = #priv, #azs = 0 (managed, without configured azs)
	// #pub = #priv = #azs (managed, all configured)
	if !cfg.managedVPCCustomized() {
		return nil
	}
	publicSubnetCIDRs := make([]string, len(cfg.Subnets.Public))
	privateSubnetCIDRs := make([]string, len(cfg.Subnets.Public))
	var azs []string

	// NOTE: sort based on `az`s to preserve the mappings between azs and public subnets, private subnets.
	// For example, if we have two subnets defined: public-subnet-1 ~ us-east-1a, and private-subnet-1 ~ us-east-1a.
	// We want to make sure that public-subnet-1, us-east-1a and private-subnet-1 are all at index 0 of in perspective lists.
	sort.SliceStable(cfg.Subnets.Public, func(i, j int) bool {
		return aws.StringValue(cfg.Subnets.Public[i].AZ) < aws.StringValue(cfg.Subnets.Public[j].AZ)
	})
	sort.SliceStable(cfg.Subnets.Private, func(i, j int) bool {
		return aws.StringValue(cfg.Subnets.Private[i].AZ) < aws.StringValue(cfg.Subnets.Private[j].AZ)
	})
	for idx, subnet := range cfg.Subnets.Public {
		publicSubnetCIDRs[idx] = aws.StringValue((*string)(subnet.CIDR))
		privateSubnetCIDRs[idx] = aws.StringValue((*string)(cfg.Subnets.Private[idx].CIDR))
		if az := aws.StringValue(subnet.AZ); az != "" {
			azs = append(azs, az)
		}
	}
	return &template.ManagedVPC{
		CIDR:               aws.StringValue((*string)(cfg.CIDR)),
		AZs:                azs,
		PublicSubnetCIDRs:  publicSubnetCIDRs,
		PrivateSubnetCIDRs: privateSubnetCIDRs,
	}
}

type subnetsConfiguration struct {
	Public  []subnetConfiguration `yaml:"public,omitempty"`
	Private []subnetConfiguration `yaml:"private,omitempty"`
}

// IsEmpty returns true if neither public subnets nor private subnets are configured.
func (cs subnetsConfiguration) IsEmpty() bool {
	return len(cs.Public) == 0 && len(cs.Private) == 0
}

type subnetConfiguration struct {
	SubnetID *string `yaml:"id,omitempty"`
	CIDR     *IPNet  `yaml:"cidr,omitempty"`
	AZ       *string `yaml:"az,omitempty"`
}

type environmentObservability struct {
	ContainerInsights *bool `yaml:"container_insights,omitempty"`
}

// IsEmpty returns true if there is no configuration to the environment's observability.
func (o *environmentObservability) IsEmpty() bool {
	return o == nil || o.ContainerInsights == nil
}

func (o *environmentObservability) loadObsConfig(tele *config.Telemetry) {
	if tele == nil {
		return
	}
	o.ContainerInsights = &tele.EnableContainerInsights
}

// EnvironmentHTTPConfig defines the configuration settings for an environment group's HTTP connections.
type EnvironmentHTTPConfig struct {
	Public  PublicHTTPConfig  `yaml:"public,omitempty"`
	Private privateHTTPConfig `yaml:"private,omitempty"`
}

// IsEmpty returns true if neither the public ALB nor the internal ALB is configured.
func (cfg EnvironmentHTTPConfig) IsEmpty() bool {
	return cfg.Public.IsEmpty() && cfg.Private.IsEmpty()
}

func (cfg *EnvironmentHTTPConfig) loadLBConfig(env *config.CustomizeEnv) {
	if env.IsEmpty() {
		return
	}

	if env.ImportVPC != nil && len(env.ImportVPC.PublicSubnetIDs) == 0 {
		cfg.Private.InternalALBSubnets = env.InternalALBSubnets
		cfg.Private.Certificates = env.ImportCertARNs
		if env.EnableInternalALBVPCIngress { // NOTE: Do not load the configuration unless it's positive, so that the default manifest does not contain the unnecessary line `http.private.ingress.vpc: false`.
			cfg.Private.Ingress.VPCIngress = aws.Bool(true)
		}
		return
	}
	cfg.Public.Certificates = env.ImportCertARNs
}

// PublicHTTPConfig represents the configuration settings for an environment public ALB.
type PublicHTTPConfig struct {
	DeprecatedSG  DeprecatedALBSecurityGroupsConfig `yaml:"security_groups,omitempty"` // Deprecated. This configuration is now available inside Ingress field.
	Certificates  []string                          `yaml:"certificates,omitempty"`
	ELBAccessLogs ELBAccessLogsArgsOrBool           `yaml:"access_logs,omitempty"`
	Ingress       RestrictiveIngress                `yaml:"ingress,omitempty"`
	SSLPolicy     *string                           `yaml:"ssl_policy,omitempty"`
}

// ELBAccessLogsArgsOrBool is a custom type which supports unmarshaling yaml which
// can either be of type bool or type ELBAccessLogsArgs.
type ELBAccessLogsArgsOrBool struct {
	Enabled        *bool
	AdvancedConfig ELBAccessLogsArgs
}

func (al *ELBAccessLogsArgsOrBool) isEmpty() bool {
	return al.Enabled == nil && al.AdvancedConfig.isEmpty()
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the ELBAccessLogsArgsOrBool
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (al *ELBAccessLogsArgsOrBool) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&al.AdvancedConfig); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !al.AdvancedConfig.isEmpty() {
		// Unmarshaled successfully to al.AccessLogsArgs, reset al.EnableAccessLogs, and return.
		al.Enabled = nil
		return nil
	}

	if err := value.Decode(&al.Enabled); err != nil {
		return errUnmarshalELBAccessLogs
	}
	return nil
}

// ELBAccessLogsArgs holds the access logs configuration.
type ELBAccessLogsArgs struct {
	BucketName *string `yaml:"bucket_name,omitempty"`
	Prefix     *string `yaml:"prefix,omitempty"`
}

func (al *ELBAccessLogsArgs) isEmpty() bool {
	return al.BucketName == nil && al.Prefix == nil
}

// ELBAccessLogs returns the access logs config if the user has set any values.
// If there is no access logs settings, then returns nil and false.
func (cfg *EnvironmentConfig) ELBAccessLogs() (*ELBAccessLogsArgs, bool) {
	accessLogs := cfg.HTTPConfig.Public.ELBAccessLogs
	if accessLogs.isEmpty() {
		return nil, false
	}
	if accessLogs.Enabled != nil {
		return nil, aws.BoolValue(accessLogs.Enabled)
	}
	return &accessLogs.AdvancedConfig, true
}

// RestrictiveIngress represents ingress fields which restrict
// default behavior of allowing all public ingress.
type RestrictiveIngress struct {
	CDNIngress *bool   `yaml:"cdn"`
	SourceIPs  []IPNet `yaml:"source_ips"`
}

// RelaxedIngress contains ingress configuration to add to a security group.
type RelaxedIngress struct {
	VPCIngress *bool `yaml:"vpc"`
}

// IsEmpty returns true if there are no specified fields for relaxed ingress.
func (i RelaxedIngress) IsEmpty() bool {
	return i.VPCIngress == nil
}

// IsEmpty returns true if there are no specified fields for restrictive ingress.
func (i RestrictiveIngress) IsEmpty() bool {
	return i.CDNIngress == nil && len(i.SourceIPs) == 0
}

// IsEmpty returns true if there is no customization to the public ALB.
func (cfg PublicHTTPConfig) IsEmpty() bool {
	return len(cfg.Certificates) == 0 && cfg.DeprecatedSG.IsEmpty() && cfg.ELBAccessLogs.isEmpty() && cfg.Ingress.IsEmpty() && cfg.SSLPolicy == nil
}

type privateHTTPConfig struct {
	InternalALBSubnets []string                          `yaml:"subnets,omitempty"`
	Certificates       []string                          `yaml:"certificates,omitempty"`
	DeprecatedSG       DeprecatedALBSecurityGroupsConfig `yaml:"security_groups,omitempty"` // Deprecated. This field is now available in Ingress.
	Ingress            RelaxedIngress                    `yaml:"ingress,omitempty"`
	SSLPolicy          *string                           `yaml:"ssl_policy,omitempty"`
}

// IsEmpty returns true if there is no customization to the internal ALB.
func (cfg privateHTTPConfig) IsEmpty() bool {
	return len(cfg.InternalALBSubnets) == 0 && len(cfg.Certificates) == 0 && cfg.DeprecatedSG.IsEmpty() && cfg.Ingress.IsEmpty() && cfg.SSLPolicy == nil
}

// HasVPCIngress returns true if the private ALB allows ingress from within the VPC.
func (cfg privateHTTPConfig) HasVPCIngress() bool {
	return aws.BoolValue(cfg.Ingress.VPCIngress) || aws.BoolValue(cfg.DeprecatedSG.DeprecatedIngress.VPCIngress)
}
