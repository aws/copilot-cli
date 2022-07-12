// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import "strconv"

// QuoteStringSlice adds quotation marks around each string in the string slice.
func QuoteStringSlice(in []string) []string {
	quoted := make([]string, len(in))
	for idx, str := range in {
		quoted[idx] = strconv.Quote(str)
	}
	return quoted
}
