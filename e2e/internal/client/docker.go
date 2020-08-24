// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"fmt"
	"strings"

	cmd "github.com/aws/copilot-cli/e2e/internal/command"
)

// Docker is a wrapper around Docker commands.
type Docker struct{}

// NewDocker returns a wrapper around Docker commands.
func NewDocker() *Docker {
	return &Docker{}
}

/*Login runs:
docker login -u AWS --password-stdin $uri
*/
func (d *Docker) Login(uri, password string) error {
	command := strings.Join([]string{
		"login",
		"-u", "AWS",
		"--password-stdin", uri,
	}, " ")
	return d.exec(command, cmd.Stdin(strings.NewReader(password)))
}

/*Build runs:
docker build -t $uri $path
*/
func (d *Docker) Build(uri, path string) error {
	command := strings.Join([]string{
		"build",
		"-t", uri, path,
	}, " ")
	return d.exec(command)
}

/*Push runs:
docker push $uri
*/
func (d *Docker) Push(uri string) error {
	command := strings.Join([]string{
		"push", uri,
	}, " ")
	return d.exec(command)
}

func (d *Docker) exec(command string, opts ...cmd.Option) error {
	return BashExec(fmt.Sprintf("docker %s", command), opts...)
}
