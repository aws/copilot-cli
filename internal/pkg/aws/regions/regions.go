// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package regions

import (
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
)

// IsServiceAvailable returns true if the service ID is available in the given region.
func IsServiceAvailable(sid string, region string) (bool, error) {
	partition, err := partitions.Region(region).Partition()
	if err != nil {
		return false, err
	}
	regions, _ := endpoints.RegionsForService(endpoints.DefaultPartitions(), partition.ID(), sid)
	if _, exist := regions[region]; exist {
		return true, nil
	}
	return false, nil
}
