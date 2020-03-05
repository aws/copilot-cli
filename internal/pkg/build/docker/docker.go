// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
)

// Service wraps a runner.
type Service struct {
	runner runner
}

type runner interface {
	Run(name string, args []string, options ...command.Option) error
}

// New returns a Service.
func New() Service {
	return Service{
		runner: command.New(),
	}
}

// Build will run a `docker build` command with the input uri, tag, and Dockerfile image path.
func (s Service) Build(uri, imageTag, path string) error {
	imageName := imageName(uri, imageTag)

	err := s.runner.Run("docker", []string{"build", "-t", imageName, path})

	if err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	return nil
}

// Login will run a `docker login` command against the Service repository URI with the input uri and auth data.
func (s Service) Login(uri, username, password string) error {
	err := s.runner.Run("docker",
		[]string{"login", "-u", username, "--password-stdin", uri},
		command.Stdin(strings.NewReader(password)))

	if err != nil {
		return fmt.Errorf("authenticate to ECR: %w", err)
	}

	return nil
}

// Push will run `docker push` command against the Service repository URI with the input uri and image tag.
func (s Service) Push(uri, imageTag string) error {
	path := imageName(uri, imageTag)

	err := s.runner.Run("docker", []string{"push", path})

	if err != nil {
		// TODO: improve the error handling here.
		// if you try to push an *existing* image that has Digest A and tag T then no error (also no image push).
		// if you try to push an *existing* image that has Digest B and tag T (that belongs to another image Digest A) then docker spits out an unclear error.
		log.Warningf("the image with tag %s may already exist.\n", imageTag)

		return fmt.Errorf("docker push: %w", err)
	}

	return nil
}

// Parse will scan the dockerfile, generate a list of ARG tokens and EXPOSE values, and attempt to intuit
// the port to be used when constructing target groups.
func (s Service) Parse(path string) (uint16, error) {
	df := newDockerfile()

	file, err := os.Open(path)
	if err != nil {
		return 80, fmt.Errorf("opening Dockerfile: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		res := strings.SplitN(scanner.Text(), " ", 2)
		df.parseLine(res)
	}
	df.aggregate()
	return df.exposedPorts[len(df.exposedPorts)-1], nil
}

type dockerfile struct {
	ambiguous     bool
	exposedPorts  []uint16
	exposedTokens []string
	tokens        map[string]string
}

func newDockerfile() *dockerfile {
	var exposedPorts []uint16
	var exposedTokens []string

	d := dockerfile{
		ambiguous:     true,
		exposedPorts:  exposedPorts,
		exposedTokens: exposedTokens,
		tokens:        make(map[string]string),
	}
	return &d
}

func (df *dockerfile) parseLine(line []string) {

	switch line[0] {
	case "ARG":
		argparts := strings.SplitN(line[1], "=", 2)
		if len(argparts) == 1 {
			df.tokens[argparts[0]] = ""
		} else if len(argparts) == 2 {
			df.tokens[argparts[0]] = argparts[1]
		} else {
		}
	case "EXPOSE":
		df.exposedTokens = append(df.exposedTokens, line[1])
	default:
	}
}

func recurse(tokenMap map[string]string, token string) (string, bool) {
	switch c := token[0]; {
	case c == '$':
		for v, ok := recurse(tokenMap, tokenMap[token]); !ok; {
			return v, false
		}
	case '0' <= c && c <= '9':
		return token, true
	}
	return token, false
}

func (df *dockerfile) aggregate() error {
	for _, tk := range df.exposedTokens {
		token, ok := recurse(df.tokens, tk)
		if ok {
			tokenUint, err := strconv.ParseUint(token, 10, 16)
			if err != nil {
				return err
			}
			df.exposedPorts = append(df.exposedPorts, uint16(tokenUint))
		}
	}
	switch len(df.exposedPorts) {
	case 1:
		df.ambiguous = false
	case 0:
		df.ambiguous = false
		return fmt.Errorf("could not determine exposed ports in dockerfile")
	default:
		df.ambiguous = true
		return fmt.Errorf("")
	}
	return nil
}

func imageName(uri, tag string) string {
	return fmt.Sprintf("%s:%s", uri, tag)
}
