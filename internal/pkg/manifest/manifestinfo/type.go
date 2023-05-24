// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifestinfo provides access to information embedded in a manifest.
package manifestinfo

const (
	// LoadBalancedWebServiceType is a web service with a load balancer and Fargate as compute.
	LoadBalancedWebServiceType = "Load Balanced Web Service"
	// RequestDrivenWebServiceType is a Request-Driven Web Service managed by AppRunner
	RequestDrivenWebServiceType = "Request-Driven Web Service"
	// BackendServiceType is a service that cannot be accessed from the internet but can be reached from other services.
	BackendServiceType = "Backend Service"
	// WorkerServiceType is a worker service that manages the consumption of messages.
	WorkerServiceType = "Worker Service"
	// StaticSiteType is a static site service that manages static assets.
	StaticSiteType = "Static Site"
	// ScheduledJobType is a recurring ECS Fargate task which runs on a schedule.
	ScheduledJobType = "Scheduled Job"
)

// ServiceTypes returns the list of supported service manifest types.
func ServiceTypes() []string {
	return []string{
		RequestDrivenWebServiceType,
		LoadBalancedWebServiceType,
		BackendServiceType,
		WorkerServiceType,
		StaticSiteType,
	}
}

// JobTypes returns the list of supported job manifest types.
func JobTypes() []string {
	return []string{
		ScheduledJobType,
	}
}

// WorkloadTypes returns the list of all manifest types.
func WorkloadTypes() []string {
	return append(ServiceTypes(), JobTypes()...)
}

// IsTypeAService returns if manifest type is service.
func IsTypeAService(t string) bool {
	for _, serviceType := range ServiceTypes() {
		if t == serviceType {
			return true
		}
	}
	return false
}

// IsTypeAJob returns if manifest type is job.
func IsTypeAJob(t string) bool {
	for _, jobType := range JobTypes() {
		if t == jobType {
			return true
		}
	}
	return false
}
