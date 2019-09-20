// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadBalancedFargateManifest_Marshal(t *testing.T) {
	// GIVEN
	wantedContent := `# Your application name will be used in naming your resources
# like log groups, services, etc.
name: SweetApp
# The "Type" of the application you're running. For a list of all types that we support see
# https://github.com/aws/PRIVATE-amazon-ecs-archer/app/template/manifest/
type: Load Balanced Web App

# The port exposed through your container. We need to know
# this so that we can route traffic to it.
containerPort: 80

# Size of CPU
cpu: 256

# Size of memory
memory: 512

# Logging is enabled by default. We'll create a loggroup that is
# the SweetApp/Stage
logging: true

# Determines whether the application will have a public IP or not.
public: true

# You can also pass in environment variables as key/value pairs
#environment-variables:
#  dog: 'Clyde'
#  cute: 'hekya'
#
# Additional Sidecar apps that can run along side your main application
#sidecars:
#  fluentbit:
#    containerPort: 80
#    image: 'amazon/aws-for-fluent-bit:1.2.0'
#    memory: 512

# This section defines each of the release stages
# and their specific configuration for your app.
stages:
  -
    # The "environment" (cluster/vpc/lb) to contain this service.
    env: test
    # The number of tasks that we want, at minimum.
    desiredCount: 1
    # Any secrets via ARNs
    #secrets:
    #  lemonaidpassword: arn:aws:secretsmanager:us-west-2:902697171733:secret:DavidsLemons/DavidsFrontEnd
  -
    # The "environment" (cluster/vpc/lb) to contain this service.
    env: prod
    # The number of tasks that we want, at minimum.
    desiredCount: 3
    # Any secrets via ARNs
    #secrets:
    #  lemonaidpassword: arn:aws:secretsmanager:us-west-2:902697171733:secret:DavidsLemons/DavidsFrontEnd
`
	m := NewLoadBalancedFargateManifest("SweetApp")
	m.Stages = append(m.Stages, AppStage{
		EnvName:      "test",
		DesiredCount: 1,
	}, AppStage{
		EnvName:      "prod",
		DesiredCount: 3,
	})

	// WHEN
	b, err := m.Marshal()

	// THEN
	require.NoError(t, err)
	require.Equal(t, wantedContent, strings.Replace(string(b), "\r\n", "\n", -1))
}
