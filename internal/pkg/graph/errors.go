// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"fmt"
	"strings"
)

type errCycle[V comparable] struct {
	vertices []V
}

func (e *errCycle[V]) Error() string {
	ss := make([]string, len(e.vertices))
	for i, v := range e.vertices {
		ss[i] = fmt.Sprintf("%v", v)
	}
	return fmt.Sprintf("graph contains a cycle: %s", strings.Join(ss, ", "))
}
