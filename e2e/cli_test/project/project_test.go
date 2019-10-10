// +build e2e

package project

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/aws/amazon-ecs-cli-v2/e2e/cli_test/internal/creds"
)

func TestArcherProjectCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(time.Now().UnixNano())
	RunSpecs(t, "Test Archer project command")
}

var cliPath string
var update = flag.Bool("update", false, "update .golden files")

var _ = BeforeSuite(func() {
	var err error
	cliPath, err = filepath.Abs("../../../bin/local/archer")
	Expect(err).To(BeNil())

	// ensure the CLI is available to e2e tests
	_, err = os.Stat(cliPath)
	Expect(err).To(BeNil())
})

var _ = Describe("Archer project command", func() {
	const projectEntity = "project"

	Context("project init", func() {
		const verb = "init"
		var (
			projectName string
			tmpDir      string
		)

		BeforeEach(func() {
			u, err := uuid.NewRandom()
			Expect(err).To(BeNil())
			projectName = "proj" + u.String()

			// create a temporary directory under the OS temporary directory
			tmpDir, err = ioutil.TempDir("", "archer-project-init")
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			err := os.RemoveAll(tmpDir)
			Expect(err).To(BeNil())
		})

		Context("succeessfully initialize a project", func() {
			var command *exec.Cmd

			AfterEach(func() {
				// set working directory to an unique temporary directory
				// so that Archer behaves like it's starting from a brand new
				// workspace
				command.Dir = tmpDir
				sess, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).To(BeNil())

				// TODO: Add assertion that lists all the projects and ensure
				// the project exists
				sess.Wait()

				exitCode := sess.ExitCode()
				Expect(exitCode).To(Equal(0))
			})

			It("should use AWS creds from environment variables", func() {
				// when command.Env is nil, the child process defaults to use
				// the parent process's environment.
				command = exec.Command(cliPath, projectEntity, verb, projectName)
			})

			It("should use shared AWS configuration file", func() {
				// dump AWS creds into a credential file
				creds, err := creds.ExtractAWSCredsFromEnvVars()
				Expect(err).To(BeNil())

				tmpFile, err := ioutil.TempFile(tmpDir, "config")
				Expect(err).To(BeNil())
				defer tmpFile.Close()

				_, err = tmpFile.Write([]byte(
					fmt.Sprintf(
						"[default]\naws_access_key_id = %s\naws_secret_access_key = %s\naws_session_token = %s\nregion = %s",
						creds.AwsAccessKey, creds.AwsSecretKey, creds.AwsSessionToken, creds.AwsRegion),
				))
				Expect(err).To(BeNil())

				command = exec.Command(cliPath, projectEntity, verb, projectName)
				// make the Archer CLI uses the credentials file
				command.Env = []string{
					fmt.Sprintf("AWS_CONFIG_FILE=%s", tmpFile.Name()),
				}
			})
		})

		Context("fails", func() {
			var command *exec.Cmd

			AfterEach(func() {
				// set working directory to an unique temporary directory
				// so that Archer behaves like it's starting from a brand new
				// workspace
				command.Dir = tmpDir
				sess, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).To(BeNil())

				actualCmdOutput := sess.Wait().Err.Contents()
				Expect(string(actualCmdOutput)).Should(MatchRegexp(`.+project.+already exists`))

				exitCode := sess.ExitCode()

				Expect(exitCode).ToNot(Equal(0))
			})

			It("should fail when attempt to re-initialize existing project", func() {
				// launch the binary to initialize a project
				firstCommand := exec.Command(cliPath, projectEntity, verb, projectName)
				firstCommand.Dir = tmpDir
				sess, err := gexec.Start(firstCommand, GinkgoWriter, GinkgoWriter)
				Expect(err).To(BeNil())

				Eventually(sess).Should(gexec.Exit(0))
				// then, attempt to initialize the same project again in AfterEach
				command = exec.Command(cliPath, projectEntity, verb, projectName)
			})
		})
	})
})
