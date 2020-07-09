// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"

// progress is the interface to inform the user that a long operation is taking place.
type progress interface {
	// Start starts displaying progress with a label.
	Start(label string)
	// Stop ends displaying progress with a label.
	Stop(label string)
	// Events writes additional information in between the start and stop stages.
	Events([]termprogress.TabRow)
}

// envProgressOrder is the order in which we want to progress text to appear on the terminal.
var envProgressOrder = []termprogress.Text{textVPC, textInternetGateway, textPublicSubnets, textPrivateSubnets, textRouteTables, textECSCluster, textALB}

// Row descriptions displayed while deploying an environment.
const (
	textVPC             termprogress.Text = "- Virtual private cloud on 2 availability zones to hold your services"
	textInternetGateway termprogress.Text = "  - Internet gateway to connect the network to the internet"
	textPublicSubnets   termprogress.Text = "  - Public subnets for internet facing services "
	textPrivateSubnets  termprogress.Text = "  - Private subnets for services that can't be reached from the internet"
	textRouteTables     termprogress.Text = "  - Routing tables for services to talk with each other"
	textECSCluster      termprogress.Text = "- ECS Cluster to hold your services "
	textALB             termprogress.Text = "- Application load balancer to distribute traffic "
)
