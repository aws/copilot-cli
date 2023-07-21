// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/aws/aws-sdk-go/aws/arn"

	"github.com/aws/aws-sdk-go/aws"
)

const (
	dashReplacement = "DASH"
	plusReplacement = "%2B"
)

// URLSafeVersion takes a Copilot version and replaces the '+'
// character with the URL-safe '%2B'.
func URLSafeVersion(version string) string {
	return strings.ReplaceAll(version, "+", plusReplacement)
}

// ReplaceDashesFunc takes a CloudFormation logical ID, and
// sanitizes it by removing "-" characters (not allowed)
// and replacing them with "DASH" (allowed by CloudFormation but
// not permitted in ecs-cli generated resource names).
func ReplaceDashesFunc(logicalID string) string {
	return strings.ReplaceAll(logicalID, "-", dashReplacement)
}

// IsARNFunc takes a string value and determines if it's an ARN or not.
func IsARNFunc(value string) bool {
	return arn.IsARN(value)
}

// TrimSlashPrefix takes a string value and removes slash prefix from the string if present.
func TrimSlashPrefix(value string) string {
	return strings.TrimPrefix(value, "/")
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

// HasCustomIngress returns true if there is any ingress specified by the customer.
func (cfg *PublicHTTPConfig) HasCustomIngress() bool {
	return len(cfg.PublicALBSourceIPs) > 0 || len(cfg.CIDRPrefixListIDs) > 0
}

// IsFIFO checks if the given queue has FIFO config.
func (s SQSQueue) IsFIFO() bool {
	return s.FIFOQueueConfig != nil
}

// EnvVarSecretFunc converts an input resource name to LogicalIDSafe, then appends
// "Secret" to the end.
func EnvVarSecretFunc(s string) string {
	return StripNonAlphaNumFunc(s) + "Secret"
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
	if len(elems) == 0 {
		return nil
	}
	quotedElems := make([]string, len(elems))
	for i, el := range elems {
		quotedElems[i] = strconv.Quote(el)
	}
	return quotedElems
}

// generateMountPointJSON turns a list of MountPoint objects into a JSON string:
// `{"myEFSVolume": "/var/www", "myEBSVolume": "/usr/data"}`
// This function must be called on an array of correctly constructed MountPoint objects.
func generateMountPointJSON(mountPoints []*MountPoint) string {
	volumeMap := make(map[string]string)

	for _, mp := range mountPoints {
		// Skip adding mount points with empty container paths to the map.
		// This is validated elsewhere so this condition should never happen, but it
		// will fail to inject mountpoints with empty paths.
		if aws.StringValue(mp.ContainerPath) == "" {
			continue
		}
		volumeMap[aws.StringValue(mp.SourceVolume)] = aws.StringValue(mp.ContainerPath)
	}

	out, ok := getJSONMap(volumeMap)
	if !ok {
		return "{}"
	}

	return string(out)

}

// generatePublisherJSON turns a list of Topics objects into a JSON string:
// `{"myTopic": "topicArn", "mySecondTopic": "secondTopicArn"}`
// This function must be called on an array of correctly constructed Topic objects.
func generateSNSJSON(topics []*Topic) string {
	if topics == nil {
		return ""
	}
	topicMap := make(map[string]string)

	for _, topic := range topics {
		// Topics with no name will not be included in the json
		if topic.Name == nil {
			continue
		}
		topicMap[aws.StringValue(topic.Name)] = topic.ARN()
	}

	out, ok := getJSONMap(topicMap)
	if !ok {
		return "{}"
	}

	return string(out)
}

// generateQueueURIJSON turns a list of Topic Subscription objects into a JSON string of their corresponding queues:
// `{"svcTopicEventsQueue": "${svctopicURL}"}`
// This function must be called on an array of correctly constructed Topic objects.
func generateQueueURIJSON(ts []*TopicSubscription) string {
	if ts == nil {
		return ""
	}
	urlMap := make(map[string]string)
	for _, sub := range ts {
		// TopicSubscriptions with no name, service, or queue will not be included in the json
		if sub.Name == nil || sub.Service == nil || sub.Queue == nil {
			continue
		}
		svc := StripNonAlphaNumFunc(aws.StringValue(sub.Service))
		topicName := StripNonAlphaNumFunc(aws.StringValue(sub.Name))
		subName := fmt.Sprintf("%s%sEventsQueue", svc, cases.Title(language.English).String(topicName))

		urlMap[subName] = fmt.Sprintf("${%s%sURL}", svc, topicName)
	}

	out, ok := getJSONMap(urlMap)
	if !ok {
		return "{}"
	}

	return string(out)
}

func getJSONMap(inMap map[string]string) ([]byte, bool) {
	// Check for empty maps
	if len(inMap) == 0 {
		return nil, false
	}

	out, err := json.Marshal(inMap)
	if err != nil {
		return nil, false
	}

	return out, true
}
