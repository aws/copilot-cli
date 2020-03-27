// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package dockerfile

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

const exposeRegexPattern = `(\d+)(\/(\w+))?` // port and optional protocol, at least 1 time on a line
const (
	exposeRegexpWholeMatch = 0
	exposeRegexpPort       = 1
	exposeRegexpProtocol   = 3
)
const reFindAllMatches = -1 // regexp package uses this as shorthand for "find all matches in string"

var (
	errCouldntParseDockerfilePort = errors.New("parse port from EXPOSE")
)

type portConfig struct {
	Port      uint16
	Protocol  string
	RawString string
}

// Dockerfile represents a parsed dockerfile
type Dockerfile struct {
	ExposedPorts []portConfig
	parsed       bool
	path         string

	fs afero.Fs
}

// New() returns an empty Dockerfile
func New(fs afero.Fs, path string) *Dockerfile {
	return &Dockerfile{
		ExposedPorts: []portConfig{},
		fs:           fs,
		path:         path,
		parsed:       false,
	}
}

// GetExposedPorts returns a uint16 slice of exposed ports found in the Dockerfile
func (df *Dockerfile) GetExposedPorts() []uint16 {
	if !df.parsed {
		df.parse()
	}

	var ports []uint16
	for _, port := range df.ExposedPorts {
		ports = append(ports, port.Port)
	}
	return ports
}

// parse takes a Dockerfile and fills in struct members based on
// methods like parseExpose and (TODO) parseHealthcheck
func (df *Dockerfile) parse() error {
	file, err := df.fs.Open(df.path)
	if err != nil {
		return fmt.Errorf("read dockerfile: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	parsedDockerfile := parseFromScanner(scanner)

	df.ExposedPorts = parsedDockerfile.ExposedPorts
	df.parsed = true
	return nil
}

func parseFromScanner(scanner *bufio.Scanner) Dockerfile {
	var line = ""
	var df Dockerfile
	df.ExposedPorts = []portConfig{}
	for scanner.Scan() {
		line = scanner.Text()
		prefix := strings.SplitN(line, " ", 2)[0]
		switch prefix {
		case "EXPOSE":
			currentPorts, _ := parseExpose(line)
			df.ExposedPorts = append(df.ExposedPorts, currentPorts...)
		}
	}
	return df
}

func parseExpose(line string) ([]portConfig, error) {
	// group 0: whole match
	// group 1: port
	// group 2: /protocol
	// group 3: protocol
	// matches strings of form <digits>(/<string>)?
	// for any number of digits and optional protocol string
	// separated by forward slash
	re, err := regexp.Compile(exposeRegexPattern)
	if err != nil {
		return []portConfig{}, err
	}

	matches := re.FindAllStringSubmatch(line, reFindAllMatches)
	// check that there are matches, if not return port with only raw data
	// there will only ever be length 0 or 4 arrays
	// TODO implement arg parser regex
	if len(matches) == 0 {
		return []portConfig{
			{
				RawString: line,
			},
		}, nil
	}
	var ports []portConfig
	for _, match := range matches {
		// convert the matched port to int and validate
		// We don't use the validate func in the cli package to avoid a circular dependency
		extractedPort, err := strconv.Atoi(match[exposeRegexpPort])
		var extractedPortUint uint16 = 0
		if err == nil && extractedPort >= 1 && extractedPort <= 65535 {
			extractedPortUint = uint16(extractedPort)
		} else {
			err = errors.New("invalid port in Dockerfile")
		}
		ports = append(ports, portConfig{
			RawString: match[exposeRegexpWholeMatch],
			Protocol:  match[exposeRegexpProtocol],
			Port:      extractedPortUint,
		})
	}
	return ports, err
}
