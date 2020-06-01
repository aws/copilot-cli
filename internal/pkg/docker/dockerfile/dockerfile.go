// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

// Package dockerfile provides simple Dockerfile parsing functionality.
package dockerfile

import (
	"errors"
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/spf13/afero"
)

var exposeRegexPattern = regexp.MustCompile(`(?P<port>\d+)(\/(?P<protocol>\w+))?`) // port and optional protocol, at least 1 time on a line

const (
	exposeRegexpWholeMatch = 0
	exposeRegexpPort       = 1
	exposeRegexpProtocol   = 3
)
const reFindAllMatches = -1 // regexp package uses this as shorthand for "find all matches in string"

const (
	intervalFlag       = "interval"
	timeoutFlag        = "timeout"
	startPeriodFlag    = "start-period"
	retriesFlag        = "retries"
	intervalDefault    = 30000000000 //30s in nanoseconds
	timeoutDefault     = 30000000000 //30s in nanoseconds
	startPeriodDefault = 0
	retriesDefault     = 3
	healthcheck        = 12
)

var (
	errCouldntParseDockerfilePort = errors.New("parse port from EXPOSE")
)

type portConfig struct {
	Port      uint16
	Protocol  string
	RawString string
	err       error
}

type healthCheck struct {
	interval    uint16
	timeout     uint16
	startPeriod uint16
	retries     uint16
	command     string
}

// Dockerfile represents a parsed Dockerfile.
type Dockerfile struct {
	ExposedPorts []portConfig
	HealthCheck  healthCheck
	parsed       bool
	path         string

	fs afero.Fs
}

// New returns an empty Dockerfile.
func New(fs afero.Fs, path string) *Dockerfile {
	return &Dockerfile{
		ExposedPorts: []portConfig{},
		HealthCheck:  healthCheck{},
		fs:           fs,
		path:         path,
		parsed:       false,
	}
}

// GetExposedPorts returns a uint16 slice of exposed ports found in the Dockerfile.
func (df *Dockerfile) GetExposedPorts() ([]uint16, error) {
	if !df.parsed {
		if err := df.parse(); err != nil {
			return nil, err
		}
	}
	var ports []uint16

	if len(df.ExposedPorts) == 0 {
		return nil, ErrNoExpose{
			Dockerfile: df.path,
		}
	}

	var err error
	for _, port := range df.ExposedPorts {
		// ensure we register that there is an error (will only be ErrNoExpose) if
		// any ports were unparseable or invalid
		if port.err != nil {
			return nil, port.err
		}
		ports = append(ports, port.Port)
	}
	return ports, err
}

// parse takes a Dockerfile and fills in struct members based on methods like parseExpose and parseHealthcheck
func (df *Dockerfile) parse() error {
	if df.parsed {
		return nil
	}

	file, err := df.fs.Open(df.path)

	if err != nil {
		return fmt.Errorf("read Dockerfile: %w", err)
	}
	defer file.Close()

	f, err := afero.ReadFile(df.fs, file.Name())
	if err != nil {
		return fmt.Errorf("read Dockerfile %s error: %w", f, err)
	}
	fileString := string(f)

	parsedDockerfile, err := parseFromReader(fileString)
	if err != nil {
		return fmt.Errorf("parse Dockerfile: %w", err)
	}

	df.ExposedPorts = parsedDockerfile.ExposedPorts
	df.HealthCheck = parsedDockerfile.HealthCheck
	df.parsed = true
	return nil
}

func parseFromReader(line string) (Dockerfile, error) {
	var df Dockerfile
	df.ExposedPorts = []portConfig{}
	df.HealthCheck = healthCheck{}

	// Parse(rwc io.Reader) reads lines from a Reader, and parses the lines into an AST.
	ast, err := parser.Parse(strings.NewReader(line))
	if err != nil {
		return df, fmt.Errorf("parse reader: %w", err)
	}

	for _, child := range ast.AST.Children {
		// ParseInstruction converts an AST to a typed instruction.
		// Does prevalidation checks before parsing
		// Example of an instruction is HEALTHCHECK CMD curl -f http://localhost/ || exit 1.
		instruction, err := instructions.ParseInstruction(child)
		if err != nil {
			return df, fmt.Errorf("parse instructions: %w", err)
		}
		inst := fmt.Sprint(instruction)

		// Getting the value at a children will return the Dockerfile directive
		switch d := child.Value; d {
		case "expose":
			currentPorts := parseExpose(inst)
			df.ExposedPorts = append(df.ExposedPorts, currentPorts...)
		case "healthcheck":
			healthCheckOptions, err := parseHealthCheck(inst)
			if err != nil {
				return df, err
			}
			df.HealthCheck = healthCheckOptions
		}
	}
	return df, nil
}

func parseExpose(line string) []portConfig {
	// group 0: whole match
	// group 1: port
	// group 2: /protocol
	// group 3: protocol
	// matches strings of form <digits>(/<string>)?
	// for any number of digits and optional protocol string
	// separated by forward slash
	matches := exposeRegexPattern.FindAllStringSubmatch(line, reFindAllMatches)

	// check that there are matches, if not return port with only raw data
	// there will only ever be length 0 or 4 arrays
	// TODO implement arg parser regex
	// https://github.com/aws/amazon-ecs-cli-v2/issues/827
	if len(matches) == 0 {
		return []portConfig{
			{
				RawString: line,
				err: ErrInvalidPort{
					Match: line,
				},
			},
		}
	}
	var ports []portConfig
	for _, match := range matches {
		var err error
		// convert the matched port to int and validate
		// We don't use the validate func in the cli package to avoid a circular dependency
		extractedPort, err := strconv.Atoi(match[exposeRegexpPort])
		if err != nil {
			ports = append(ports, portConfig{
				err: ErrInvalidPort{
					Match: match[0],
				},
			})
			continue
		}
		var extractedPortUint uint16
		if extractedPort >= 1 && extractedPort <= 65535 {
			extractedPortUint = uint16(extractedPort)
		} else {
			err = ErrInvalidPort{Match: match[0]}
		}
		ports = append(ports, portConfig{
			RawString: match[exposeRegexpWholeMatch],
			Protocol:  match[exposeRegexpProtocol],
			Port:      extractedPortUint,
			err:       err,
		})
	}
	return ports
}

func parseHealthCheck(line string) (healthCheck, error) {
	var hc healthCheck

	if line[healthcheck:] == "NONE" {
		return healthCheck{}, nil
	}

	var retries int
	var interval, timeout, startPeriod time.Duration
	fs := flag.NewFlagSet("flags", flag.ContinueOnError)

	fs.DurationVar(&interval, intervalFlag, intervalDefault, "")
	fs.DurationVar(&timeout, timeoutFlag, timeoutDefault, "")
	fs.DurationVar(&startPeriod, startPeriodFlag, startPeriodDefault, "")
	fs.IntVar(&retries, retriesFlag, retriesDefault, "")

	if err := fs.Parse(strings.Split(line[healthcheck:], " ")); err != nil {
		return healthCheck{}, err
	}

	hc = healthCheck{
		interval:    uint16(interval.Seconds()),
		timeout:     uint16(timeout.Seconds()),
		startPeriod: uint16(startPeriod.Seconds()),
		retries:     uint16(retries),
		command:     regexp.MustCompile("CMD.*").FindString(line),
	}
	return hc, nil
}
