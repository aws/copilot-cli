// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package partitions

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/endpoints"
)

// Region is an AWS region ID.
type Region string

// Partition returns the first partition which includes the region passed in, from a list of the partitions the SDK is bundled with.
func (r Region) Partition() (endpoints.Partition, error) {
	partition, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), string(r))
	if !ok {
		return endpoints.Partition{}, fmt.Errorf("find the partition for region %s", string(r))
	}
	return partition, nil
}

// IsAvailableInRegion returns true if the service ID is available in the given region.
func IsAvailableInRegion(sID string, region string) (bool, error) {
	partition, err := Region(region).Partition()
	if err != nil {
		return false, err
	}
	regions, partitionOrServiceExists := endpoints.RegionsForService(endpoints.DefaultPartitions(), partition.ID(), sID)
	if !partitionOrServiceExists {
		return false, nil
	}
	_, existInRegion := regions[region]
	return existInRegion, nil
}
