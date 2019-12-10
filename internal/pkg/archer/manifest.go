// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Manifest is the interface for serializing a manifest object to a YAML document or CloudFormation template.
type Manifest interface {
	Marshal() ([]byte, error)
	DockerfilePath() string
	AppName() string
	IntegTestBuildspecPath() string
}
