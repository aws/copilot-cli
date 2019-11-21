// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"fmt"
	"runtime"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/version"
	"github.com/aws/aws-sdk-go/aws/request"
)

const UserAgentHeader = "User-Agent"

// UserAgentHandler returns a http request handler that sets a custom user agent to all aws requests.
func UserAgentHandler() request.NamedHandler {
	return request.NamedHandler{
		Name: "UserAgentHandler",
		Fn: func(r *request.Request) {
			userAgent := r.HTTPRequest.Header.Get(UserAgentHeader)
			r.HTTPRequest.Header.Set(UserAgentHeader,
				fmt.Sprintf("aws-ecs-cli-v2/%s %s (%s) %s", version.Version, version.Platform, runtime.GOOS, userAgent))
		},
	}
}