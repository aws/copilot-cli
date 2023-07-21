// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import "github.com/aws/copilot-cli/internal/pkg/template"

type loadBalancedWebSvcReadParser interface {
	template.ReadParser
	ParseLoadBalancedWebService(template.WorkloadOpts) (*template.Content, error)
}

type backendSvcReadParser interface {
	template.ReadParser
	ParseBackendService(template.WorkloadOpts) (*template.Content, error)
}

type requestDrivenWebSvcReadParser interface {
	template.ReadParser
	ParseRequestDrivenWebService(template.WorkloadOpts) (*template.Content, error)
}

type workerSvcReadParser interface {
	template.ReadParser
	ParseWorkerService(template.WorkloadOpts) (*template.Content, error)
}

type staticSiteReadParser interface {
	template.ReadParser
	ParseStaticSite(template.WorkloadOpts) (*template.Content, error)
}

type scheduledJobReadParser interface {
	template.ReadParser
	ParseScheduledJob(template.WorkloadOpts) (*template.Content, error)
}

type envReadParser interface {
	template.ReadParser
	ParseEnv(data *template.EnvOpts) (*template.Content, error)
	ParseEnvBootstrap(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error)
}

type pipelineParser interface {
	ParsePipeline(data interface{}) (*template.Content, error)
}

// embedFS is the interface to parse any embedded templates.
type embedFS interface {
	backendSvcReadParser
	loadBalancedWebSvcReadParser
	requestDrivenWebSvcReadParser
	staticSiteReadParser
	scheduledJobReadParser
	workerSvcReadParser
	envReadParser
}

var (
	realEmbedFS embedFS = template.New()
	fs                  = realEmbedFS
)
