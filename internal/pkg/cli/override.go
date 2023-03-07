// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

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
