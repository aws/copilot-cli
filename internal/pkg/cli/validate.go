// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/spf13/afero"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

var (
	errValueEmpty                         = errors.New("value must not be empty")
	errValueTooLong                       = errors.New("value must not exceed 255 characters")
	errValueBadFormat                     = errors.New("value must start with a letter, contain only lower-case letters, numbers, and hyphens, and have no consecutive or trailing hyphen")
	errValueNotAString                    = errors.New("value must be a string")
	errValueNotAStringSlice               = errors.New("value must be a string slice")
	errValueNotAValidPath                 = errors.New("value must be a valid path")
	errValueNotAnIPNet                    = errors.New("value must be a valid IP address range (example: 10.0.0.0/16)")
	errValueNotIPNetSlice                 = errors.New("value must be a valid slice of IP address range (example: 10.0.0.0/16,10.0.1.0/16)")
	errPortInvalid                        = errors.New("value must be in range 1-65535")
	errS3ValueBadSize                     = errors.New("value must be between 3 and 63 characters in length")
	errS3ValueBadFormat                   = errors.New("value must not contain consecutive periods or dashes, or be formatted as IP address")
	errS3ValueTrailingDash                = errors.New("value must not have trailing -")
	errValueBadFormatWithPeriod           = errors.New("value must contain only lowercase alphanumeric characters and .-")
	errDDBValueBadSize                    = errors.New("value must be between 3 and 255 characters in length")
	errDDBAttributeBadSize                = errors.New("value must be between 1 and 255 characters in length")
	errValueBadFormatWithPeriodUnderscore = errors.New("value must contain only alphanumeric characters and ._-")
	errDDBAttributeBadFormat              = errors.New("value must be of the form <name>:<T> where T is one of S, N, or B")
	errTooManyLSIKeys                     = errors.New("number of specified LSI sort keys must be 5 or less")
	errDomainInvalid                      = errors.New("value must contain at least one '.' character")
	errDurationInvalid                    = errors.New("value must be a valid Go duration string (example: 1h30m)")
	errDurationBadUnits                   = errors.New("duration cannot be in units smaller than a second")
	errScheduleInvalid                    = errors.New("value must be a valid cron expression (examples: @weekly; @every 30m; 0 0 * * 0)")

	// Aurora-Serverless-specific errors.
	errInvalidRDSNameCharacters = errors.New("value must start with a letter")
)

var (
	fmtErrInvalidStorageType = "invalid storage type %s: must be one of %s"

	// Aurora-Serverless-specific errors.
	fmtErrRDSNameBadSize          = "value must be between %d and %d characters in length"
	fmtErrInvalidEngineType       = "invalid engine type %s: must be one of %s"
	fmtErrInvalidDBNameCharacters = "invalid database name %s: must contain only alphanumeric characters and underscore; should start with a letter"
)

var (
	emptyIPNet = net.IPNet{}
	emptyIP    = net.IP{}
)

// matches alphanumeric, ._-, from 3 to 255 characters long
// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/HowItWorks.NamingRulesDataTypes.html
var ddbRegExp = regexp.MustCompile(`^[a-zA-Z0-9\-\.\_]+$`)

// s3 validation expressions.
// s3RegExp matches alphanumeric, .- from 3 to 63 characters long.
// punctuationRegExp matches consecutive dashes or periods.
// trailingPunctRegExp matches a trailing dash.
// ipAddressRegExp checks for a bucket in the format of an IP address.
// https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-s3-bucket-naming-requirements.html
// The punctuation and trailing punctuation guidelines also apply to ECR repositories, though
// the requirements are not documented.
var (
	s3RegExp = regexp.MustCompile("" +
		`^` + // start of line.
		`[a-z0-9\.\-]{3,63}` + // Main match: lowercase alphanumerics, ., - from 3-63 characters.
		`$`, // end of line.
	)
	punctuationRegExp = regexp.MustCompile(
		`[\.\-]{2,}`, // Check for consecutive periods or dashes.
	)
	trailingPunctRegExp = regexp.MustCompile(
		`[\-\.]$`, // Check for trailing dash or dot.
	)
	ipAddressRegexp = regexp.MustCompile(
		`^(?:\d{1,3}\.){3}\d{1,3}$`, // Match any 1-3 digits in xxx.xxx.xxx.xxx format.
	)

	domainNameRegexp = regexp.MustCompile(`\.`) // Check for at least one dot in domain name.

	awsScheduleRegexp = regexp.MustCompile(`(?:rate|cron)\(.*\)`) // Check for strings of the form rate(*) or cron(*).
)

// RDS Aurora Serverless validation expressions.
var (
	// Referred to name constraints here: https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.CreateInstance.html#Aurora.CreateInstance.Settings
	// However, the doc on name constraints is somewhat misleading.
	// PostgreSQL db name cannot start with an underscore (doc says it must begin with a letter or an underscore).
	// MySQL db name can contain underscores (not limited to alphanumeric as described in the doc).
	dbNameCharacterRegExp = regexp.MustCompile("" +
		"^" + // Start of string.
		"[A-Za-z]" + // Starts with a letter.
		"[0-9A-Za-z_]*" + // Subsequent characters can be letters, underscores or digits
		"$", // End of string.
	)

	// The storage name for RDS storage type is used as the logical ID of the Aurora Serverless DB cluster in the CFN template.
	// When creating the DB cluster, CFN will use the logical ID to generate a DB cluster identifier.
	// Since the logical ID has stricter character restrictions than cluster identifier, we only need to check if the
	// starting character is a letter.
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.CreateInstance.html#Aurora.CreateInstance.Settings
	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html
	rdsStorageNameRegExp = regexp.MustCompile("" +
		"^" + // Start of string.
		"[A-Za-z]" + // Starts with a letter. The DB cluster identifier must start with a letter.
		`[a-zA-Z0-9\-\.\_]*` + // Followed by alphanumeric, ._-. Refers to POSIX portable file name character set.
		"$", // End of string.
	)
)

const regexpFindAllMatches = -1

func validateAppName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("application name %v is invalid: %w", val, err)
	}
	return nil
}

func validateSvcName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("service name %v is invalid: %w", val, err)
	}
	return nil
}

func validateSvcPort(val interface{}) error {

	if err := basicPortValidation(val); err != nil {
		return fmt.Errorf("port %v is invalid: %w", val, err)
	}
	return nil
}

func validateSvcType(val interface{}) error {
	svcType, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	return validateWorkloadType(svcType, manifest.ServiceTypes, service)
}

func validateWorkloadType(wkldType string, validTypes []string, errFlavor string) error {
	for _, validType := range validTypes {
		if wkldType == validType {
			return nil
		}
	}

	return fmt.Errorf("invalid %s type %s: must be one of %s", errFlavor, wkldType, prettify(validTypes))
}

func validateJobType(val interface{}) error {
	jobType, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	return validateWorkloadType(jobType, manifest.JobTypes, job)
}

func validateJobName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("job name %v is invalid: %w", val, err)
	}
	return nil
}

func validateSchedule(sched interface{}) error {
	s, ok := sched.(string)
	if !ok {
		return errValueNotAString
	}
	return validateCron(s)
}

func validateTimeout(timeout interface{}) error {
	t, ok := timeout.(string)
	if !ok {
		return errValueNotAString
	}
	if err := validateDuration(t, 1*time.Second); err != nil {
		return fmt.Errorf("timeout value %s is invalid: %w", t, err)
	}
	return nil
}

func validateRate(rate interface{}) error {
	r, ok := rate.(string)
	if !ok {
		return errValueNotAString
	}
	return validateDuration(r, 60*time.Second)
}

func validateDomainName(val interface{}) error {
	domainName, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	dots := domainNameRegexp.FindAllString(domainName, regexpFindAllMatches)
	if dots == nil {
		return errDomainInvalid
	}
	return nil
}

func validatePath(fs afero.Fs, val interface{}) error {
	path, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if path == "" {
		return errValueEmpty
	}
	_, err := fs.Stat(path)
	if err != nil {
		return errValueNotAValidPath
	}
	return nil
}

func validateStorageType(val interface{}) error {
	storageType, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	for _, validType := range storageTypes {
		if storageType == validType {
			return nil
		}
	}
	return fmt.Errorf(fmtErrInvalidStorageType, storageType, prettify(storageTypes))
}

func validateMySQLDBName(val interface{}) error {
	const (
		minMySQLDBNameLength = 1
		maxMySQLDBNameLength = 64
	)

	dbName, ok := val.(string)
	if !ok {
		return errValueNotAString
	}

	// Check for db name length.
	if len(dbName) < minMySQLDBNameLength || len(dbName) > maxMySQLDBNameLength {
		return fmt.Errorf(fmtErrRDSNameBadSize, minMySQLDBNameLength, maxMySQLDBNameLength)
	}

	return validateDBNameCharacters(dbName)
}

func validatePostgreSQLDBName(val interface{}) error {
	const (
		minPostgreSQLDBNameLength = 1
		maxPostgreSQLDBNameLength = 63
	)

	dbName, ok := val.(string)
	if !ok {
		return errValueNotAString
	}

	// Check for db name length.
	if len(dbName) < minPostgreSQLDBNameLength || len(dbName) > maxPostgreSQLDBNameLength {
		return fmt.Errorf(fmtErrRDSNameBadSize, minPostgreSQLDBNameLength, maxPostgreSQLDBNameLength)
	}

	return validateDBNameCharacters(dbName)
}

func validateDBNameCharacters(name string) error {
	// Check for character constraints.
	match := dbNameCharacterRegExp.FindStringSubmatch(name)
	if match != nil {
		return nil
	}

	return fmt.Errorf(fmtErrInvalidDBNameCharacters, name)
}

func validateEngine(val interface{}) error {
	engine, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	for _, valid := range engineTypes {
		if engine == valid {
			return nil
		}
	}
	return fmt.Errorf(fmtErrInvalidEngineType, engine, prettify(engineTypes))
}

func validateEnvironmentName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("environment name %v is invalid: %w", val, err)
	}
	return nil
}

func basicNameValidation(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if s == "" {
		return errValueEmpty
	}
	if len(s) > 255 {
		return errValueTooLong
	}
	if !isCorrectFormat(s) {
		return errValueBadFormat
	}

	return nil
}

func validateCron(sched string) error {
	// If the schedule is wrapped in aws terms `rate()` or `cron()`, don't validate it--
	// instead, pass it in as-is for serverside validation. AWS cron is weird (year field, nonstandard wildcards)
	// so for edge cases we need to support it
	awsSchedMatch := awsScheduleRegexp.FindStringSubmatch(sched)
	if awsSchedMatch != nil {
		return nil
	}
	every := "@every "
	if strings.HasPrefix(sched, every) {
		if err := validateDuration(sched[len(every):], 60*time.Second); err != nil {
			if err == errDurationInvalid {
				return fmt.Errorf("interval %s must include a valid Go duration string (example: @every 1h30m)", sched)
			}
			return fmt.Errorf("interval %s is invalid: %s", sched, err)
		}
	}
	_, err := cron.ParseStandard(sched)
	if err != nil {
		return fmt.Errorf("schedule %s is invalid: %s", sched, errScheduleInvalid)
	}
	return nil
}

func validateDuration(duration string, min time.Duration) error {
	parsedDuration, err := time.ParseDuration(duration)
	if err != nil {
		return errDurationInvalid
	}
	// This checks if the duration has parts smaller than a whole second.
	if parsedDuration > parsedDuration.Truncate(time.Second) {
		return errDurationBadUnits
	}
	if parsedDuration < min {
		return fmt.Errorf("duration must be %v or greater", min)
	}
	return nil
}

func isCorrectFormat(s string) bool {
	valid, err := regexp.MatchString(`^[a-z][a-z0-9\-]+$`, s)
	if err != nil {
		return false // bubble up error?
	}

	// Check for bad punctuation (no consecutive dashes or dots)
	formatMatch := punctuationRegExp.FindStringSubmatch(s)
	if len(formatMatch) != 0 {
		return false
	}

	trailingMatch := trailingPunctRegExp.FindStringSubmatch(s)
	if len(trailingMatch) != 0 {
		return false
	}

	return valid
}

func basicPortValidation(val interface{}) error {

	var err error

	switch val := val.(type) {
	case []byte:
		err = bytePortValidation(val)
	case string:
		err = stringPortValidation(val)
	case uint16:
		if val == 0 {
			err = errPortInvalid
		}
	default:
		err = errPortInvalid
	}
	return err
}

func bytePortValidation(val []byte) error {
	s := string(val)
	err := stringPortValidation(s)
	if err != nil {
		return err
	}
	return nil
}

func stringPortValidation(val string) error {
	port64, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return errPortInvalid
	}
	if port64 < 1 || port64 > 65535 {
		return errPortInvalid
	}
	return nil
}

// s3 bucket names: 'a-z0-9.-'
func s3BucketNameValidation(val interface{}) error {
	const minS3Length = 3
	const maxS3Length = 63
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if len(s) < minS3Length || len(s) > maxS3Length {
		return errS3ValueBadSize
	}

	// check for correct character set
	nameMatch := s3RegExp.FindStringSubmatch(s)
	if len(nameMatch) == 0 {
		return errValueBadFormatWithPeriod
	}

	// Check for bad punctuation (no consecutive dashes or dots)
	formatMatch := punctuationRegExp.FindStringSubmatch(s)
	if len(formatMatch) != 0 {
		return errS3ValueBadFormat
	}

	dashMatch := trailingPunctRegExp.FindStringSubmatch(s)
	if len(dashMatch) != 0 {
		return errS3ValueTrailingDash
	}

	ipMatch := ipAddressRegexp.FindStringSubmatch(s)
	if len(ipMatch) != 0 {
		return errS3ValueBadFormat
	}

	return nil
}

// Dynamo table names: 'a-zA-Z0-9.-_'
func dynamoTableNameValidation(val interface{}) error {
	// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/HowItWorks.NamingRulesDataTypes.html
	const minDDBTableNameLength = 3
	const maxDDBTableNameLength = 255

	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if len(s) < minDDBTableNameLength || len(s) > maxDDBTableNameLength {
		return errDDBValueBadSize
	}
	m := ddbRegExp.FindStringSubmatch(s)
	if len(m) == 0 {
		return errValueBadFormatWithPeriodUnderscore
	}
	return nil
}

// Dynamo attribute names: 1 to 255 characters
func dynamoAttributeNameValidation(val interface{}) error {
	// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/HowItWorks.NamingRulesDataTypes.html
	const minDDBAttributeNameLength = 1
	const maxDDBAttributeNameLength = 255

	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if len(s) < minDDBAttributeNameLength || len(s) > maxDDBAttributeNameLength {
		return errDDBAttributeBadSize
	}
	return nil
}

// RDS storage name: '[a-zA-Z][a-zA-Z0-9]*'
func rdsNameValidation(val interface{}) error {
	// This length constrains needs to satisfy: 1. logical ID length; 2. DB Cluster identifier length.
	// For 1. logical ID, there is no documented length limit.
	// For 2. DB Cluster identifier, the maximal length is 63.
	// DB Cluster identifier is auto-generated by CFN using the cluster's logical ID, which is the storage name appended
	// by "DBCluster". Hence the maximal length of the storage name is 63 - len("DBCluster")
	const minRDSNameLength = 1
	const maxRDSNameLength = 63 - len("DBCluster")

	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if len(s) < minRDSNameLength || len(s) > maxRDSNameLength {
		return fmt.Errorf(fmtErrRDSNameBadSize, minRDSNameLength, maxRDSNameLength)
	}
	m := rdsStorageNameRegExp.FindStringSubmatch(s)
	if m == nil {
		return errInvalidRDSNameCharacters
	}
	return nil
}

func validateKey(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	attr, err := addon.DDBAttributeFromKey(s)
	if err != nil {
		return errDDBAttributeBadFormat
	}
	err = dynamoAttributeNameValidation(*attr.Name)
	if err != nil {
		return errDDBAttributeBadSize
	}
	err = validateDynamoDataType(*attr.DataType)
	if err != nil {
		return err
	}
	return nil
}

func validateDynamoDataType(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if !strings.Contains("SNB", strings.ToUpper(s)) {
		return errDDBAttributeBadFormat
	}
	return nil
}

func validateLSIs(val interface{}) error {
	s, ok := val.([]string)
	if !ok {
		return errValueNotAStringSlice
	}
	if len(s) > 5 {
		return errTooManyLSIKeys
	}
	for _, att := range s {
		err := validateKey(att)
		if err != nil {
			return err
		}
	}
	return nil
}

func prettify(inputStrings []string) string {
	prettyTypes := template.QuoteSliceFunc(inputStrings)
	return strings.Join(prettyTypes, ", ")
}

func validateCIDR(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	ip, _, err := net.ParseCIDR(s)
	if err != nil || ip.String() == emptyIP.String() {
		return errValueNotAnIPNet
	}
	return nil
}

func validateCIDRSlice(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	slice := strings.Split(s, ",")
	if len(slice) == 0 {
		return errValueNotIPNetSlice
	}
	for _, str := range slice {
		if err := validateCIDR(str); err != nil {
			return errValueNotIPNetSlice
		}
	}
	return nil
}
