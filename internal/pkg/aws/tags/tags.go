// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package tags implements simple functions to manipulate AWS resource tags.
package tags

// Merge creates and returns a new map by adding tags in the order provided.
func Merge(tags ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, t := range tags {
		for k, v := range t {
			merged[k] = v
		}
	}
	return merged
}
