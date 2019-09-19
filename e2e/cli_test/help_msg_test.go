// +build e2e isolated

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("archer help messages", func() {
	var exitCode int

	AfterEach(func() {
		Expect(exitCode).To(Equal(0))
	})

	Context("top-level help message", func() {
		var (
			expectedHelpMsgFile     = filepath.Join("testdata", "top-level-help-msg.golden")
			expectedToplevelHelpMsg []byte
			actualHelpMsg           []byte
		)

		BeforeEach(func() {
			var err error
			expectedToplevelHelpMsg, err = ioutil.ReadFile(expectedHelpMsgFile)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			if *update {
				ioutil.WriteFile(expectedHelpMsgFile, actualHelpMsg, 0644)
			}

			Expect(string(actualHelpMsg)).To(Equal(string(expectedToplevelHelpMsg)))
		})

		It("should print top-level help message when run with no argument", func() {
			command := exec.Command(cliPath)
			sess, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).To(BeNil())

			actualHelpMsg = sess.Wait().Out.Contents()
			exitCode = sess.ExitCode()
		})

		It("should print top-level help message when run with -h", func() {
			command := exec.Command(cliPath, "-h")
			sess, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).To(BeNil())

			actualHelpMsg = sess.Wait().Out.Contents()
			exitCode = sess.ExitCode()
		})
	})
})
