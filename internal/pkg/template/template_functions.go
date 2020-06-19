// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	dashReplacement = "DASH"
)

// ReplaceDashes takes a CloudFormation logical ID, and
// sanitizes it by removing "-" characters (not allowed)
// and replacing them with "DASH" (allowed by CloudFormation but
// not permitted in ecs-cli generated resource names).
func ReplaceDashes(logicalID string) string {
	return strings.ReplaceAll(logicalID, "-", dashReplacement)
}

// DashReplacedLogicalIDToOriginal takes a "sanitized" logical ID
// and converts it back to its original form, with dashes.
func DashReplacedLogicalIDToOriginal(safeLogicalID string) string {
	return strings.ReplaceAll(safeLogicalID, dashReplacement, "-")
}

var nonAlphaNum = regexp.MustCompile("[^a-zA-Z0-9]+")

// StorageLogicalIDSafe strips non-alphanumeric characters from an input string.
func StorageLogicalIDSafe(s string) string {
	return nonAlphaNum.ReplaceAllString(s, "")
}

// EnvVarName converts an input resource name to LogicalIDSafe, then appends
// "Name" to the end. When generating environment variables, this string
// is then passed through the "toSnakeCase" method
func EnvVarName(s string) string {
	return StorageLogicalIDSafe(s) + "Name"
}

// Grabs word boundaries in default CamelCase. Matches lowercase letters & numbers
// before the next capital as capturing group 1, and the first capital in the
// next word as capturing group 2. Will match "yC" in "MyCamel" and "y2ndC" in"My2ndCamel"
var lowerUpperRegexp = regexp.MustCompile("([a-z0-9]+)([A-Z])")

// Grabs word boundaries of the form "DD[B][In]dex". Matches the last uppercase
// letter of an acronym as CG1, and the next uppercase + lowercase combo (indicating
// a new word) as CG2. Will match "BTa" in"MyDDBTableWithLSI" or "2Wi" in"MyDDB2WithLSI"
var upperLowerRegexp = regexp.MustCompile("([A-Z0-9])([A-Z][a-z])")

// ToSnakeCase transforms a CamelCase input string s into an upper SNAKE_CASE string and returns it.
// For example, "usersDdbTableName" becomes "USERS_DDB_TABLE_NAME".
func ToSnakeCase(s string) string {
	sSnake := lowerUpperRegexp.ReplaceAllString(s, "${1}_${2}")
	sSnake = upperLowerRegexp.ReplaceAllString(sSnake, "${1}_${2}")
	return strings.ToUpper(sSnake)
}

// Inc increments an integer value and returns the result.
func Inc(i int) int { return i + 1 }

// FmtSlice renders a string representation of a go string slice, surrounded by brackets
// and joined by commas.
func FmtSlice(elems []string) string {
	return fmt.Sprintf("[%s]", strings.Join(elems, ", "))
}

// QuoteSlice places quotation marks around all elements of a go string slice.
func QuoteSlice(elems []string) []string {
	quotedElems := make([]string, len(elems))
	for i, el := range elems {
		quotedElems[i] = strconv.Quote(el)
	}
	return quotedElems
}
