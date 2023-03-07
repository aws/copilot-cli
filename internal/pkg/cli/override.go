// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"strings"
)

const (
	// IaC options for overrides.
	cdkIaCTool = "cdk"

	// IaC toolkit configuration.
	typescriptCDKLang = "typescript"
)

var validIaCTools = []string{
	cdkIaCTool,
}

var validCDKLangs = []string{
	typescriptCDKLang,
}

type stringWriteCloser interface {
	fmt.Stringer
	io.WriteCloser
}

type closableStringBuilder struct {
	*strings.Builder
}

// Close implements the io.Closer interface for a strings.Builder and is a no-op.
func (sb *closableStringBuilder) Close() error {
	return nil
}
