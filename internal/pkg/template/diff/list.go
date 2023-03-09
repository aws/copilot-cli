// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

func longestCommonSubsequence[T comparable](a []T, b []T) []T {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	// Initialize the matrix
	lcs := make([][]int, len(a)+1)
	for i := 0; i < len(a)+1; i++ {
		lcs[i] = make([]int, len(b)+1)
		lcs[i][len(b)] = 0
	}
	for j := 0; j < len(b)+1; j++ {
		lcs[len(a)][j] = 0
	}
	// Compute the lengths of the LCS for all sub lists.
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = 1 + lcs[i+1][j+1]
				continue
			}
			if lcs[i+1][j] < lcs[i][j+1] {
				lcs[i][j] = lcs[i][j+1]
			} else {
				lcs[i][j] = lcs[i+1][j]
			}
		}
	}
	// Backtrace to construct the LCS.
	var i, j int
	var seq []T
	for {
		if i >= len(a) || j >= len(b) {
			break
		}
		if a[i] == b[j] {
			seq = append(seq, a[i])
			i++
			j++
			continue
		}
		if lcs[i+1][j] < lcs[i][j+1] {
			j++
		} else {
			i++
		}
	}
	return seq
}
