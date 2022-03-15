// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package dockerfile provides functionality to parse a Dockerfile.
package dockerfile

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	intervalFlag    = "interval"
	intervalDefault = 10 * time.Second

	timeoutFlag    = "timeout"
	timeoutDefault = 5 * time.Second

	startPeriodFlag    = "start-period"
	startPeriodDefault = 0

	hcRetriesFlag  = "retries"
	retriesDefault = 2

	cmdInstructionPrefix = "CMD "
	cmdShell             = "CMD-SHELL"
)

// Port represents an exposed port in a Dockerfile.
type Port struct {
	Port      uint16
	Protocol  string
	RawString string
	err       error
}

// String implements the fmt.Stringer interface.
func (p *Port) String() string {
	return p.RawString
}

// HealthCheck represents container health check options in a Dockerfile.
type HealthCheck struct {
	Interval    time.Duration
	Timeout     time.Duration
	StartPeriod time.Duration
	Retries     int
	Cmd         []string
}

// Dockerfile represents a parsed Dockerfile.
type Dockerfile struct {
	exposedPorts []Port
	healthCheck  *HealthCheck
	parsed       bool
	path         string

	fs afero.Fs
}

// New returns an empty Dockerfile.
func New(fs afero.Fs, path string) *Dockerfile {
	return &Dockerfile{
		exposedPorts: []Port{},
		healthCheck:  nil,
		fs:           fs,
		path:         path,
	}
}

// GetExposedPorts returns a uint16 slice of exposed ports found in the Dockerfile.
func (df *Dockerfile) GetExposedPorts() ([]Port, error) {
	if !df.parsed {
		if err := df.parse(); err != nil {
			return nil, err
		}
	}
	if len(df.exposedPorts) == 0 {
		return nil, ErrNoExpose{
			Dockerfile: df.path,
		}
	}

	var portsWithoutErrs []Port
	for _, port := range df.exposedPorts {
		// ensure we register that there is an error (will only be ErrNoExpose) if
		// any ports were unparseable or invalid
		if port.err != nil {
			return nil, port.err
		}
		portsWithoutErrs = append(portsWithoutErrs, port)
	}
	return portsWithoutErrs, nil
}

// GetHealthCheck parses the HEALTHCHECK instruction from the Dockerfile and returns it.
// If the HEALTHCHECK is NONE or there is no instruction, returns nil.
func (df *Dockerfile) GetHealthCheck() (*HealthCheck, error) {
	if !df.parsed {
		if err := df.parse(); err != nil {
			return nil, err
		}
	}
	return df.healthCheck, nil
}

// parse takes a Dockerfile and fills in struct members based on methods like parseExpose and parseHealthcheck.
func (df *Dockerfile) parse() error {
	if df.parsed {
		return nil
	}

	file, err := df.fs.Open(df.path)

	if err != nil {
		return fmt.Errorf("open Dockerfile: %w", err)
	}
	defer file.Close()

	f, err := afero.ReadFile(df.fs, file.Name())
	if err != nil {
		return fmt.Errorf("read Dockerfile %s error: %w", f, err)
	}

	parsedDockerfile, err := parse(file.Name(), string(f))
	if err != nil {
		return err
	}

	df.exposedPorts = parsedDockerfile.exposedPorts
	df.healthCheck = parsedDockerfile.healthCheck
	df.parsed = true
	return nil
}

// parse parses the contents of a Dockerfile into a Dockerfile struct.
func parse(name, content string) (*Dockerfile, error) {
	var df Dockerfile
	df.exposedPorts = []Port{}

	lexer := lex(strings.NewReader(content))
	for {
		instr := lexer.next()
		switch instr.name {
		case instrErr:
			return nil, fmt.Errorf("scan Dockerfile %s: %s", name, instr.args)
		case instrEOF:
			return &df, nil
		case instrExpose:
			currentPorts := parseExpose(instr.args)
			df.exposedPorts = append(df.exposedPorts, currentPorts...)
		case instrHealthCheck:
			hc, err := parseHealthCheck(instr.args)
			if err != nil {
				return nil, err
			}
			df.healthCheck = hc
		}
	}
}

func parseExpose(line string) []Port {
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
	// https://github.com/aws/copilot-cli/issues/827
	if len(matches) == 0 {
		return []Port{
			{
				RawString: line,
				err: ErrInvalidPort{
					Match: line,
				},
			},
		}
	}
	var ports []Port
	for _, match := range matches {
		var err error
		// convert the matched port to int and validate
		// We don't use the validate func in the cli package to avoid a circular dependency
		extractedPort, err := strconv.Atoi(match[exposeRegexpPort])
		if err != nil {
			ports = append(ports, Port{
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
		ports = append(ports, Port{
			RawString: match[exposeRegexpWholeMatch],
			Protocol:  match[exposeRegexpProtocol],
			Port:      extractedPortUint,
			err:       err,
		})
	}
	return ports
}

// parseHealthCheck takes a HEALTHCHECK directives and turns into a healthCheck struct.
func parseHealthCheck(content string) (*HealthCheck, error) {
	if strings.ToUpper(strings.TrimSpace(content)) == "NONE" {
		return nil, nil
	}
	if !strings.Contains(content, "CMD") {
		return nil, errors.New("parse HEALTHCHECK: instruction must contain either CMD or NONE")
	}

	var retries int
	var interval, timeout, startPeriod time.Duration
	fs := flag.NewFlagSet("flags", flag.ContinueOnError)
	fs.DurationVar(&interval, intervalFlag, intervalDefault, "")
	fs.DurationVar(&timeout, timeoutFlag, timeoutDefault, "")
	fs.DurationVar(&startPeriod, startPeriodFlag, startPeriodDefault, "")
	fs.IntVar(&retries, hcRetriesFlag, retriesDefault, "")

	var instrArgs []string
	for _, arg := range strings.Split(content, " ") {
		if arg == "" {
			continue
		}
		instrArgs = append(instrArgs, strings.TrimSpace(arg))
	}

	if err := fs.Parse(instrArgs); err != nil {
		return nil, fmt.Errorf("parse HEALTHCHECK: %w", err)
	}

	// if HEALTHCHECK instruction is not "NONE", there must be a "CMD" instruction otherwise will error out.
	// The CMD instruction can either be in a shell command format: `HEALTHCHECK CMD /bin/check-running`
	// Or, it can also be an exec array: HEALTHCHECK CMD ["/bin/check-running"]
	cmdIndex := strings.Index(content, cmdInstructionPrefix)
	cmdArgs := strings.TrimSpace(content[cmdIndex+len(cmdInstructionPrefix):])
	cmdExecutor := "CMD"
	var args []string
	if err := json.Unmarshal([]byte(cmdArgs), &args); err != nil {
		cmdExecutor = cmdShell // In string form, use CMD-SHELL.
		args = []string{cmdArgs}
	}

	return &HealthCheck{
		Interval:    interval,
		Timeout:     timeout,
		StartPeriod: startPeriod,
		Retries:     retries,
		Cmd:         append([]string{cmdExecutor}, args...),
	}, nil
}
