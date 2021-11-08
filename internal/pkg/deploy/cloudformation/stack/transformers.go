// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"hash/crc32"
	"strings"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template/override"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

const (
	enabled  = "ENABLED"
	disabled = "DISABLED"
)

// Default values for EFS options
const (
	defaultRootDirectory   = "/"
	defaultIAM             = disabled
	defaultReadOnly        = true
	defaultWritePermission = false
)

// Supported capacityproviders for Fargate services
const (
	capacityProviderFargateSpot = "FARGATE_SPOT"
	capacityProviderFargate     = "FARGATE"
)

var (
	taskDefOverrideRulePrefixes = []string{"Resources", "TaskDefinition", "Properties"}
)

// convertSidecar converts the manifest sidecar configuration into a format parsable by the templates pkg.
func convertSidecar(s map[string]*manifest.SidecarConfig) ([]*template.SidecarOpts, error) {
	if s == nil {
		return nil, nil
	}
	var sidecars []*template.SidecarOpts
	for name, config := range s {
		port, protocol, err := parsePortMapping(config.Port)
		if err != nil {
			return nil, err
		}
		entrypoint, err := convertEntryPoint(config.EntryPoint)
		if err != nil {
			return nil, err
		}
		command, err := convertCommand(config.Command)
		if err != nil {
			return nil, err
		}
		mp := convertSidecarMountPoints(config.MountPoints)
		sidecars = append(sidecars, &template.SidecarOpts{
			Name:       aws.String(name),
			Image:      config.Image,
			Essential:  config.Essential,
			Port:       port,
			Protocol:   protocol,
			CredsParam: config.CredsParam,
			Secrets:    config.Secrets,
			Variables:  config.Variables,
			Storage: template.SidecarStorageOpts{
				MountPoints: mp,
			},
			DockerLabels: config.DockerLabels,
			DependsOn:    convertDependsOn(config.DependsOn),
			EntryPoint:   entrypoint,
			HealthCheck:  convertContainerHealthCheck(config.HealthCheck),
			Command:      command,
		})
	}
	return sidecars, nil
}

func convertContainerHealthCheck(hc manifest.ContainerHealthCheck) *template.ContainerHealthCheck {
	if hc.IsEmpty() {
		return nil
	}
	// Make sure that unset fields in the healthcheck gets a default value.
	hc.ApplyIfNotSet(manifest.NewDefaultContainerHealthCheck())
	return &template.ContainerHealthCheck{
		Command:     hc.Command,
		Interval:    aws.Int64(int64(hc.Interval.Seconds())),
		Retries:     aws.Int64(int64(aws.IntValue(hc.Retries))),
		StartPeriod: aws.Int64(int64(hc.StartPeriod.Seconds())),
		Timeout:     aws.Int64(int64(hc.Timeout.Seconds())),
	}
}

// convertDependsOn converts image and sidecar depends on fields to have upper case statuses.
func convertDependsOn(d manifest.DependsOn) map[string]string {
	if d == nil {
		return nil
	}
	dependsOn := make(map[string]string)
	for name, status := range d {
		dependsOn[name] = strings.ToUpper(status)
	}
	return dependsOn
}

// Valid sidecar portMapping example: 2000/udp, or 2000 (default to be tcp).
func parsePortMapping(s *string) (port *string, protocol *string, err error) {
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

func convertAdvancedCount(a manifest.AdvancedCount) (*template.AdvancedCount, error) {
	if a.IsEmpty() {
		return nil, nil
	}
	autoscaling, err := convertAutoscaling(a)
	if err != nil {
		return nil, err
	}
	return &template.AdvancedCount{
		Spot:        a.Spot,
		Autoscaling: autoscaling,
		Cps:         convertCapacityProviders(a),
	}, nil
}

// convertCapacityProviders transforms the manifest fields into a format
// parsable by the templates pkg.
func convertCapacityProviders(a manifest.AdvancedCount) []*template.CapacityProviderStrategy {
	if a.IsEmpty() {
		return nil
	}
	// return if autoscaling range specified without spot scaling
	if !a.Range.IsEmpty() && a.Range.Value != nil {
		return nil
	}
	var cps []*template.CapacityProviderStrategy
	// if Spot specified as count, then weight on Spot CPS should be 1
	cps = append(cps, &template.CapacityProviderStrategy{
		Weight:           aws.Int(1),
		CapacityProvider: capacityProviderFargateSpot,
	})
	// Return if only spot is specifed as count
	if a.Range.IsEmpty() {
		return cps
	}
	// Scaling with spot
	rc := a.Range.RangeConfig
	if !rc.IsEmpty() {
		spotFrom := aws.IntValue(rc.SpotFrom)
		min := aws.IntValue(rc.Min)
		// If spotFrom value is not equal to the autoscaling min, then
		// the base value on the Fargate Capacity provider must be set
		// to one less than spotFrom
		if spotFrom > min {
			base := spotFrom - 1
			fgCapacity := &template.CapacityProviderStrategy{
				Base:             aws.Int(base),
				Weight:           aws.Int(0),
				CapacityProvider: capacityProviderFargate,
			}
			cps = append(cps, fgCapacity)
		}
	}
	return cps
}

// convertAutoscaling converts the service's Auto Scaling configuration into a format parsable
// by the templates pkg.
func convertAutoscaling(a manifest.AdvancedCount) (*template.AutoscalingOpts, error) {
	if a.IsEmpty() {
		return nil, nil
	}
	if a.Spot != nil {
		return nil, nil
	}

	min, max, err := a.Range.Parse()
	if err != nil {
		return nil, err
	}
	autoscalingOpts := template.AutoscalingOpts{
		MinCapacity: &min,
		MaxCapacity: &max,
	}
	if a.CPU != nil {
		autoscalingOpts.CPU = aws.Float64(float64(*a.CPU))
	}
	if a.Memory != nil {
		autoscalingOpts.Memory = aws.Float64(float64(*a.Memory))
	}
	if a.Requests != nil {
		autoscalingOpts.Requests = aws.Float64(float64(*a.Requests))
	}
	if a.ResponseTime != nil {
		responseTime := float64(*a.ResponseTime) / float64(time.Second)
		autoscalingOpts.ResponseTime = aws.Float64(responseTime)
	}
	if !a.QueueScaling.IsEmpty() {
		acceptableBacklog, err := a.QueueScaling.AcceptableBacklogPerTask()
		if err != nil {
			return nil, err
		}
		autoscalingOpts.QueueDelay = &template.AutoscalingQueueDelayOpts{
			AcceptableBacklogPerTask: acceptableBacklog,
		}
	}
	return &autoscalingOpts, nil
}

// convertHTTPHealthCheck converts the ALB health check configuration into a format parsable by the templates pkg.
func convertHTTPHealthCheck(hc *manifest.HealthCheckArgsOrString) template.HTTPHealthCheckOpts {
	opts := template.HTTPHealthCheckOpts{
		HealthCheckPath:    manifest.DefaultHealthCheckPath,
		HealthyThreshold:   hc.HealthCheckArgs.HealthyThreshold,
		UnhealthyThreshold: hc.HealthCheckArgs.UnhealthyThreshold,
		GracePeriod:        aws.Int64(manifest.DefaultHealthCheckGracePeriod),
	}
	if hc.HealthCheckArgs.Path != nil {
		opts.HealthCheckPath = *hc.HealthCheckArgs.Path
	} else if hc.HealthCheckPath != nil {
		opts.HealthCheckPath = *hc.HealthCheckPath
	}
	if hc.HealthCheckArgs.SuccessCodes != nil {
		opts.SuccessCodes = *hc.HealthCheckArgs.SuccessCodes
	}
	if hc.HealthCheckArgs.Interval != nil {
		opts.Interval = aws.Int64(int64(hc.HealthCheckArgs.Interval.Seconds()))
	}
	if hc.HealthCheckArgs.Timeout != nil {
		opts.Timeout = aws.Int64(int64(hc.HealthCheckArgs.Timeout.Seconds()))
	}
	if hc.HealthCheckArgs.GracePeriod != nil {
		opts.GracePeriod = aws.Int64(int64(hc.HealthCheckArgs.GracePeriod.Seconds()))
	}
	return opts
}

// TODO: implement this after manifest package is updated with `nlb`.
func convertNetworkLoadBalancer() *template.NetworkLoadBalancer {
	return nil
}

func convertExecuteCommand(e *manifest.ExecuteCommand) *template.ExecuteCommandOpts {
	if e.Config.IsEmpty() && !aws.BoolValue(e.Enable) {
		return nil
	}
	return &template.ExecuteCommandOpts{}
}

func convertLogging(lc manifest.Logging) *template.LogConfigOpts {
	if lc.IsEmpty() {
		return nil
	}
	return &template.LogConfigOpts{
		Image:          lc.LogImage(),
		ConfigFile:     lc.ConfigFile,
		EnableMetadata: lc.GetEnableMetadata(),
		Destination:    lc.Destination,
		SecretOptions:  lc.SecretOptions,
		Variables:      lc.Variables,
		Secrets:        lc.Secrets,
	}
}

func convertTaskDefOverrideRules(inRules []manifest.OverrideRule) []override.Rule {
	var res []override.Rule
	suffixStr := strings.Join(taskDefOverrideRulePrefixes, override.PathSegmentSeparator)
	for _, r := range inRules {
		res = append(res, override.Rule{
			Path:  strings.Join([]string{suffixStr, r.Path}, override.PathSegmentSeparator),
			Value: r.Value,
		})
	}
	return res
}

// convertStorageOpts converts a manifest Storage field into template data structures which can be used
// to execute CFN templates
func convertStorageOpts(wlName *string, in manifest.Storage) *template.StorageOpts {
	if in.IsEmpty() {
		return nil
	}
	return &template.StorageOpts{
		Ephemeral:         convertEphemeral(in.Ephemeral),
		Volumes:           convertVolumes(in.Volumes),
		MountPoints:       convertMountPoints(in.Volumes),
		EFSPerms:          convertEFSPermissions(in.Volumes),
		ManagedVolumeInfo: convertManagedFSInfo(wlName, in.Volumes),
	}
}

func convertEphemeral(in *int) *int {
	// Min value for extensible ephemeral storage is 21; if customer specifies 20, which is the default size,
	// we shouldn't let CF error out. Instead, we'll just omit it from the config.
	if aws.IntValue(in) == 20 {
		return nil
	}
	return in
}

// convertSidecarMountPoints is used to convert from manifest to template objects.
func convertSidecarMountPoints(in []manifest.SidecarMountPoint) []*template.MountPoint {
	if len(in) == 0 {
		return nil
	}
	var output []*template.MountPoint
	for _, smp := range in {
		output = append(output, convertMountPoint(smp.SourceVolume, smp.ContainerPath, smp.ReadOnly))
	}
	return output
}

func convertMountPoint(sourceVolume, containerPath *string, readOnly *bool) *template.MountPoint {
	// readOnly defaults to true.
	oReadOnly := aws.Bool(defaultReadOnly)
	if readOnly != nil {
		oReadOnly = readOnly
	}
	return &template.MountPoint{
		ReadOnly:      oReadOnly,
		ContainerPath: containerPath,
		SourceVolume:  sourceVolume,
	}
}

func convertMountPoints(input map[string]*manifest.Volume) []*template.MountPoint {
	if len(input) == 0 {
		return nil
	}
	var output []*template.MountPoint
	for name, volume := range input {
		output = append(output, convertMountPoint(aws.String(name), volume.ContainerPath, volume.ReadOnly))
	}
	return output
}

func convertEFSPermissions(input map[string]*manifest.Volume) []*template.EFSPermission {
	var output []*template.EFSPermission
	for _, volume := range input {
		// If there's no EFS configuration, we don't need to generate any permissions.
		if volume.EmptyVolume() {
			continue
		}
		// If EFS is explicitly disabled, we don't need to generate permisisons.
		if volume.EFS.Disabled() {
			continue
		}
		// Managed FS permissions are rendered separately in the template.
		if volume.EFS.UseManagedFS() {
			continue
		}

		// Write defaults to false.
		write := defaultWritePermission
		if volume.ReadOnly != nil {
			write = !aws.BoolValue(volume.ReadOnly)
		}
		accessPointID := volume.EFS.Advanced.AuthConfig.AccessPointID
		output = append(output, &template.EFSPermission{
			Write:         write,
			AccessPointID: accessPointID,
			FilesystemID:  volume.EFS.Advanced.FileSystemID,
		})
	}
	return output
}

func convertManagedFSInfo(wlName *string, input map[string]*manifest.Volume) *template.ManagedVolumeCreationInfo {
	var output *template.ManagedVolumeCreationInfo
	for name, volume := range input {
		if volume.EmptyVolume() || !volume.EFS.UseManagedFS() {
			continue
		}
		uid := volume.EFS.Advanced.UID
		gid := volume.EFS.Advanced.GID
		if uid == nil && gid == nil {
			crc := aws.Uint32(getRandomUIDGID(wlName))
			uid = crc
			gid = crc
		}
		output = &template.ManagedVolumeCreationInfo{
			Name:    aws.String(name),
			DirName: wlName,
			UID:     uid,
			GID:     gid,
		}
	}
	return output
}

// getRandomUIDGID returns the 32-bit checksum of the service name for use as CreationInfo in the EFS Access Point.
// See https://stackoverflow.com/a/14210379/5890422 for discussion of the possibility of collisions in CRC32 with
// small numbers of hashes.
func getRandomUIDGID(name *string) uint32 {
	return crc32.ChecksumIEEE([]byte(aws.StringValue(name)))
}

func convertVolumes(input map[string]*manifest.Volume) []*template.Volume {
	var output []*template.Volume
	for name, volume := range input {
		// Volumes can contain either:
		//   a) an EFS configuration, which must be valid
		//   b) no EFS configuration, in which case the volume is created using task scratch storage in order to share
		//      data between containers.

		// If EFS is not configured, just add the name to create an empty volume and continue.
		if volume.EmptyVolume() {
			output = append(
				output,
				&template.Volume{
					Name: aws.String(name),
				},
			)
			continue
		}

		// If we're using managed EFS, continue.
		if volume.EFS.UseManagedFS() {
			continue
		}

		// Convert EFS configuration to template struct.
		output = append(
			output,
			&template.Volume{
				Name: aws.String(name),
				EFS:  convertEFSConfiguration(volume.EFS.Advanced),
			},
		)
	}
	return output
}

func convertEFSConfiguration(in manifest.EFSVolumeConfiguration) *template.EFSVolumeConfiguration {
	// Set default values correctly.
	rootDir := in.RootDirectory
	if aws.StringValue(rootDir) == "" {
		rootDir = aws.String(defaultRootDirectory)
	}
	// Set default values for IAM and AccessPointID
	iam := aws.String(defaultIAM)
	if in.AuthConfig.IsEmpty() {
		return &template.EFSVolumeConfiguration{
			Filesystem:    in.FileSystemID,
			RootDirectory: rootDir,
			IAM:           iam,
		}
	}
	// AuthConfig exists; check the properties.
	if aws.BoolValue(in.AuthConfig.IAM) {
		iam = aws.String(enabled)
	}

	return &template.EFSVolumeConfiguration{
		Filesystem:    in.FileSystemID,
		RootDirectory: rootDir,
		IAM:           iam,
		AccessPointID: in.AuthConfig.AccessPointID,
	}
}

func convertNetworkConfig(network manifest.NetworkConfig) template.NetworkOpts {
	if network.IsEmpty() {
		return template.NetworkOpts{
			AssignPublicIP: template.EnablePublicIP,
			SubnetsType:    template.PublicSubnetsPlacement,
		}
	}
	opts := template.NetworkOpts{
		AssignPublicIP: template.EnablePublicIP,
		SubnetsType:    template.PublicSubnetsPlacement,
		SecurityGroups: network.VPC.SecurityGroups,
	}
	if network.VPC.Placement == nil {
		return opts
	}
	if *network.VPC.Placement != manifest.PublicSubnetPlacement {
		opts.AssignPublicIP = template.DisablePublicIP
		opts.SubnetsType = template.PrivateSubnetsPlacement
	}
	return opts
}

func convertRDWSNetworkConfig(network manifest.RequestDrivenWebServiceNetworkConfig) template.NetworkOpts {
	opts := template.NetworkOpts{}
	if network.IsEmpty() {
		return opts
	}
	if network.VPC.Placement == nil {
		return opts
	}
	if string(*network.VPC.Placement) == string(manifest.PrivateSubnetPlacement) {
		opts.SubnetsType = template.PrivateSubnetsPlacement
	}
	return opts
}

func convertAlias(alias manifest.Alias) ([]string, error) {
	out, err := alias.ToStringSlice()
	if err != nil {
		return nil, fmt.Errorf(`convert "http.alias" to string slice: %w`, err)
	}
	return out, nil
}

func convertEntryPoint(entrypoint manifest.EntryPointOverride) ([]string, error) {
	out, err := entrypoint.ToStringSlice()
	if err != nil {
		return nil, fmt.Errorf(`convert "entrypoint" to string slice: %w`, err)
	}
	return out, nil
}

func convertCommand(command manifest.CommandOverride) ([]string, error) {
	out, err := command.ToStringSlice()
	if err != nil {
		return nil, fmt.Errorf(`convert "command" to string slice: %w`, err)
	}
	return out, nil
}

func convertPublish(topics []manifest.Topic, accountID, region, app, env, svc string) (*template.PublishOpts, error) {
	if len(topics) == 0 {
		return nil, nil
	}
	partition, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	if !ok {
		return nil, fmt.Errorf("find the partition for region %s", region)
	}
	var publishers template.PublishOpts
	// convert the topics to template Topics
	for _, topic := range topics {
		publishers.Topics = append(publishers.Topics, &template.Topic{
			Name:      topic.Name,
			AccountID: accountID,
			Partition: partition.ID(),
			Region:    region,
			App:       app,
			Env:       env,
			Svc:       svc,
		})
	}

	return &publishers, nil
}

func convertSubscribe(s manifest.SubscribeConfig, accountID, region, app, env, svc string) (*template.SubscribeOpts, error) {
	if s.Topics == nil {
		return nil, nil
	}
	sqsEndpoint, err := endpoints.DefaultResolver().EndpointFor(endpoints.SqsServiceID, region)
	if err != nil {
		return nil, err
	}
	var subscriptions template.SubscribeOpts
	for _, sb := range s.Topics {
		ts := convertTopicSubscription(sb, sqsEndpoint.URL, accountID, app, env, svc)
		subscriptions.Topics = append(subscriptions.Topics, ts)
	}
	subscriptions.Queue = convertQueue(s.Queue)
	return &subscriptions, nil
}

func convertTopicSubscription(t manifest.TopicSubscription, url, accountID, app, env, svc string) *template.TopicSubscription {
	if aws.BoolValue(t.Queue.Enabled) {
		return &template.TopicSubscription{
			Name:    t.Name,
			Service: t.Service,
			Queue:   &template.SQSQueue{},
		}
	}
	return &template.TopicSubscription{
		Name:    t.Name,
		Service: t.Service,
		Queue:   convertQueue(t.Queue.Advanced),
	}
}

func convertQueue(q manifest.SQSQueue) *template.SQSQueue {
	if q.IsEmpty() {
		return nil
	}
	return &template.SQSQueue{
		Retention:  convertRetention(q.Retention),
		Delay:      convertDelay(q.Delay),
		Timeout:    convertTimeout(q.Timeout),
		DeadLetter: convertDeadLetter(q.DeadLetter),
	}
}

func convertTime(t *time.Duration) *int64 {
	if t == nil {
		return nil
	}
	return aws.Int64(int64(t.Seconds()))
}

func convertRetention(t *time.Duration) *int64 {
	return convertTime(t)
}

func convertDelay(t *time.Duration) *int64 {
	return convertTime(t)
}

func convertTimeout(t *time.Duration) *int64 {
	return convertTime(t)
}

func convertDeadLetter(d manifest.DeadLetterQueue) *template.DeadLetterQueue {
	if d.IsEmpty() {
		return nil
	}
	return &template.DeadLetterQueue{
		Tries: d.Tries,
	}
}

func parseS3URLs(nameToS3URL map[string]string) (bucket *string, s3ObjectKeys map[string]*string, err error) {
	if len(nameToS3URL) == 0 {
		return nil, nil, nil
	}
	s3ObjectKeys = make(map[string]*string)
	for fname, s3url := range nameToS3URL {
		bucketName, key, err := s3.ParseURL(s3url)
		if err != nil {
			return nil, nil, err
		}
		s3ObjectKeys[fname] = &key
		bucket = &bucketName
	}
	return
}

func convertAppInformation(app deploy.AppInformation) (delegationRole *string, dnsName *string) {
	role := app.DNSDelegationRole()
	if role != "" {
		delegationRole = &role
	}
	dns := app.DNSName
	if dns != "" {
		dnsName = &dns
	}
	return
}

func convertPlatform(platform manifest.PlatformArgsOrString) template.RuntimePlatformOpts {
	if platform.IsEmpty() {
		return template.RuntimePlatformOpts{}
	}

	os := template.OSLinux
	switch platform.OS() {
	case manifest.OSWindows, manifest.OSWindowsServer2019Core:
		os = template.OSWindowsServerCore
	case manifest.OSWindowsServer2019Full:
		os = template.OSWindowsServerFull
	}
	return template.RuntimePlatformOpts{
		OS:   os,
		Arch: template.ArchX86,
	}
}
