// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package partitions

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/endpoints"
)

type Region string

// Partition returns the first partition which includes the region passed in, from a list of the partitions the SDK is bundled with.
func (r Region) Partition() (endpoints.Partition, error) {
	partition, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), string(r))
	if !ok {
		return endpoints.Partition{}, fmt.Errorf("find the partition for region %s", string(r))
	}
	return partition, nil
}
