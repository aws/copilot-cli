// +build e2e

package project

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestArcherProjectCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(time.Now().UnixNano())
	RunSpecs(t, "Test Archer project command")
}

var cliPath string

var _ = BeforeSuite(func() {
	// ensure the e2e tests are performed on the latest code changes by
	// compiling CLI from source
	var err error
	cliPath, err = gexec.Build("../../../cmd/archer/main.go")
	Expect(err).Should(BeNil())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
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
			u, err := uuid.NewV4()
			Expect(err).To(BeNil())
			projectName = u.String()

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

				actualCmdOutput := sess.Wait().Err.Contents()
				Expect(string(actualCmdOutput)).To(ContainSubstring("Created Project"))

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
				creds, err := extractAWSCredsFromEnvVars()
				Expect(err).To(BeNil())

				tmpFile, err := ioutil.TempFile(tmpDir, "config")
				Expect(err).To(BeNil())
				defer tmpFile.Close()

				_, err = tmpFile.Write([]byte(
					fmt.Sprintf(
						"[default]\naws_access_key_id = %s\naws_secret_access_key = %s\naws_session_token = %s\nregion = %s",
						creds.awsAccessKey, creds.awsSecretKey, creds.awsSessionToken, creds.awsRegion),
				))
				Expect(err).To(BeNil())

				command = exec.Command(cliPath, projectEntity, verb, projectName)
				// make the Archer CLI uses the credentials file
				command.Env = []string{
					fmt.Sprintf("AWS_CONFIG_FILE=%s", tmpFile.Name()),
				}

				fmt.Println("Name", tmpFile.Name())
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

type creds struct {
	awsAccessKey    string
	awsSecretKey    string
	awsSessionToken string
	awsRegion       string
}

func extractAWSCredsFromEnvVars() (*creds, error) {
	const (
		awsAccessKeyName        = "AWS_ACCESS_KEY_ID"
		awsSecretKeyName        = "AWS_SECRET_ACCESS_KEY"
		awsSessionTokenName     = "AWS_SESSION_TOKEN"
		awsDefaultRegionKeyName = "AWS_DEFAULT_REGION"
	)

	var res = &creds{}

	for _, pair := range os.Environ() {
		fmt.Println(pair)
		keyValue := strings.SplitN(pair, "=", 2)
		if len(keyValue) < 2 {
			return nil, errors.New("invalid environment variable format")
		}
		key := keyValue[0]
		value := keyValue[1]

		if key == awsAccessKeyName {
			res.awsAccessKey = value
		} else if key == awsSecretKeyName {
			res.awsSecretKey = value
		} else if key == awsSessionTokenName {
			res.awsSessionToken = value
		} else if key == awsDefaultRegionKeyName {
			res.awsRegion = value
		} else {
			continue
		}
	}

	if res.awsAccessKey == "" || res.awsSecretKey == "" || res.awsSessionToken == "" || res.awsRegion == "" {
		return nil, fmt.Errorf(
			"failed to parse AWS credentials out of environment variables, "+
				"AWS_ACCESS_KEY_ID=%s AWS_SECRET_ACCESS_KEY=%s "+
				"AWS_SESSION_TOKEN=%s AWS_DEFAULT_REGION=%s",
			res.awsAccessKey, res.awsSecretKey, res.awsSessionToken,
			res.awsRegion,
		)
	}

	return res, nil
}
