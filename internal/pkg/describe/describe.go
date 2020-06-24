// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/dustin/go-humanize"
	"io"
)

const (
	// Display settings.
	minCellWidth           = 20  // minimum number of characters in a table's cell.
	tabWidth               = 4   // number of characters in between columns.
	cellPaddingWidth       = 2   // number of padding characters added by default to a cell.
	paddingChar            = ' ' // character in between columns.
	noAdditionalFormatting = 0
)

// humanizeTime is overriden in tests so that its output is constant as time passes.
var humanizeTime = humanize.Time

// HumanJSONStringer contains methods that stringify app info for output.
type HumanJSONStringer interface {
	HumanString() string
	JSONString() (string, error)
}

type cfnResources map[string][]*CfnResource

// CfnResource contains application resources created by cloudformation.
type CfnResource struct {
	Type       string `json:"type"`
	PhysicalID string `json:"physicalID"`
}

func flattenResources(stackResources []*cloudformation.StackResource) []*CfnResource {
	var resources []*CfnResource
	for _, stackResource := range stackResources {
		resources = append(resources, &CfnResource{
			Type:       aws.StringValue(stackResource.ResourceType),
			PhysicalID: aws.StringValue(stackResource.PhysicalResourceId),
		})
	}
	return resources
}

func (c CfnResource) HumanString() string {
	return fmt.Sprintf("    %s\t%s\n", c.Type, c.PhysicalID)
}

func (c cfnResources) humanStringByEnv(w io.Writer, configs []*ServiceConfig) {
	// Go maps don't have a guaranteed order.
	// Show the resources by the order of environments displayed under Configuration for a consistent view.
	for _, config := range configs {
		env := config.Environment
		resources := c[env]
		fmt.Fprintf(w, "\n  %s\n", env)
		for _, resource := range resources {
			fmt.Fprintf(w, "    %s\t%s\n", resource.Type, resource.PhysicalID)
		}
	}
}
