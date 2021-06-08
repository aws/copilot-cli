// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package pipeline_test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("pipeline flow", func() {
	Context("setup CodeCommit repository", func() {
		var cloneURL string
		It("creates the codecommit repository", func() {
			url, err := aws.CreateCodeCommitRepo(repoName)
			Expect(err).NotTo(HaveOccurred())
			cloneURL = url
		})

		It("clones the repository", func() {
			endpoint := strings.TrimPrefix(cloneURL, "https://")
			url := fmt.Sprintf("https://%s:%s@%s", url.PathEscape(codeCommitCreds.UserName), url.PathEscape(codeCommitCreds.Password), endpoint)

			Eventually(func() error {
				cmd := exec.Command("git", "clone", url)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				return cmd.Run()
			}, "60s", "5s").ShouldNot(HaveOccurred())
		})

		It("copies source code to the git repository", func() {
			cmd := exec.Command("cp", "-r", "frontend", repoName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			Expect(cmd.Run()).NotTo(HaveOccurred())
		})

		It("should push upstream", func() {
			cmd := exec.Command("git", "add", ".")
			cmd.Dir = repoName
			Expect(cmd.Run()).NotTo(HaveOccurred())

			cmd = exec.Command("git", "commit", "-m", "first commit")
			cmd.Dir = repoName
			Expect(cmd.Run()).NotTo(HaveOccurred())

			cmd = exec.Command("git", "push")
			cmd.Dir = repoName
			Expect(cmd.Run()).NotTo(HaveOccurred())
		})
	})
})
