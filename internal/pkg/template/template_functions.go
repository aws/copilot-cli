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

// ReplaceDashesFunc takes a CloudFormation logical ID, and
// sanitizes it by removing "-" characters (not allowed)
// and replacing them with "DASH" (allowed by CloudFormation but
// not permitted in ecs-cli generated resource names).
func ReplaceDashesFunc(logicalID string) string {
	return strings.ReplaceAll(logicalID, "-", dashReplacement)
}

// DashReplacedLogicalIDToOriginal takes a "sanitized" logical ID
// and converts it back to its original form, with dashes.
func DashReplacedLogicalIDToOriginal(safeLogicalID string) string {
	return strings.ReplaceAll(safeLogicalID, dashReplacement, "-")
}

var nonAlphaNum = regexp.MustCompile("[^a-zA-Z0-9]+")

// StripNonAlphaNumFunc strips non-alphanumeric characters from an input string.
func StripNonAlphaNumFunc(s string) string {
	return nonAlphaNum.ReplaceAllString(s, "")
}

// EnvVarNameFunc converts an input resource name to LogicalIDSafe, then appends
// "Name" to the end.
func EnvVarNameFunc(s string) string {
	return StripNonAlphaNumFunc(s) + "Name"
}

// Grabs word boundaries in default CamelCase. Matches lowercase letters & numbers
// before the next capital as capturing group 1, and the first capital in the
// next word as capturing group 2. Will match "yC" in "MyCamel" and "y2ndC" in"My2ndCamel"
var lowerUpperRegexp = regexp.MustCompile("([a-z0-9]+)([A-Z])")

// Grabs word boundaries of the form "DD[B][In]dex". Matches the last uppercase
// letter of an acronym as CG1, and the next uppercase + lowercase combo (indicating
// a new word) as CG2. Will match "BTa" in"MyDDBTableWithLSI" or "2Wi" in"MyDDB2WithLSI"
var upperLowerRegexp = regexp.MustCompile("([A-Z0-9])([A-Z][a-z])")

// ToSnakeCaseFunc transforms a CamelCase input string s into an upper SNAKE_CASE string and returns it.
// For example, "usersDdbTableName" becomes "USERS_DDB_TABLE_NAME".
func ToSnakeCaseFunc(s string) string {
	sSnake := lowerUpperRegexp.ReplaceAllString(s, "${1}_${2}")
	sSnake = upperLowerRegexp.ReplaceAllString(sSnake, "${1}_${2}")
	return strings.ToUpper(sSnake)
}

// IncFunc increments an integer value and returns the result.
func IncFunc(i int) int { return i + 1 }

// FmtSliceFunc renders a string representation of a go string slice, surrounded by brackets
// and joined by commas.
func FmtSliceFunc(elems []string) string {
	return fmt.Sprintf("[%s]", strings.Join(elems, ", "))
}

// QuoteSliceFunc places quotation marks around all elements of a go string slice.
func QuoteSliceFunc(elems []string) []string {
	var quotedElems []string
	if len(elems) == 0 {
		return quotedElems
	}
	quotedElems = make([]string, len(elems))
	for i, el := range elems {
		quotedElems[i] = strconv.Quote(el)
	}
	return quotedElems
}

// QuotePSliceFunc places quotation marks around all
// dereferenced elements of elems and returns a []string slice.
func QuotePSliceFunc(elems []*string) []string {
	var quotedElems []string
	if len(elems) == 0 {
		return quotedElems
	}
	quotedElems = make([]string, len(elems))
	for i, el := range elems {
		quotedElems[i] = strconv.Quote(*el)
	}
	return quotedElems
}
