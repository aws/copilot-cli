// +build e2e

package cli_test

import (
	"os/exec"

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
		const expectedToplevelHelpMsg = "Launch and manage applications on Amazon ECS and AWS Fargate 🚀"
		var actualHelpMsg []byte

		AfterEach(func() {
			Expect(string(actualHelpMsg)).
				To(ContainSubstring(expectedToplevelHelpMsg))
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
