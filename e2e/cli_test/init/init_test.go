// +build e2e

package init

import (
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
)

const (
	typeLoadBalancedWebApp = "Load Balanced Web App"
)

func TestArcherInitCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(time.Now().UnixNano())
	RunSpecs(t, "Test Archer init command")
}

var cliPath string

var _ = BeforeSuite(func() {
	var err error
	cliPath, err = filepath.Abs("../../../bin/local/ecs-preview")
	Expect(err).To(BeNil())

	// ensure the CLI is available for e2e tests
	_, err = os.Stat(cliPath)
	Expect(err).To(BeNil())
})

var _ = Describe("Archer init command", func() {
	const verb = "init"
	var (
		tmpDir  string
		command *exec.Cmd
	)

	BeforeEach(func() {
		// create a temporary directory under the OS-dependent temp folder
		var err error
		tmpDir, err = ioutil.TempDir("", "archer-init")
		Expect(err).To(BeNil())

		// when command.Env is nil, the child process defaults to use
		// the parent process's environment, so all AWS creds will be inherited
		// from the environment variables of the test process
		command = exec.Command(cliPath)
		// set working directory to an unique temporary directory
		// so that Archer behaves like it's starting from a brand new
		// workspace
		command.Dir = tmpDir
	})

	AfterEach(func() {
		err := os.RemoveAll(tmpDir)
		Expect(err).To(BeNil())
	})

	Context("init succeeds", func() {
		const (
			manifestFolder  = "ecs-project"
			summaryFileName = ".ecs-workspace"
		)
		var (
			sess        *gexec.Session
			projectName string
		)

		BeforeEach(func() {
			u, err := uuid.NewRandom()
			Expect(err).To(BeNil())
			projectName = "e2einit" + u.String()
		})

		AfterEach(func() {
			exitCode := sess.ExitCode()
			Expect(exitCode).To(Equal(0))
		})

		It("e2e without deployment", func() {
			command.Args = append(command.Args,
				"init",
				"--project", projectName,
				"--app", "CoolApp",
				"--app-type", typeLoadBalancedWebApp,
				"--deploy=false",
			)

			var err error
			sess, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).To(BeNil())
			sess.Wait()

			// assert that the expected files should be created after
			// successfully executed the `archer init` command.
			_, err = os.Stat(filepath.Join(tmpDir, manifestFolder))
			Expect(err).To(BeNil())
			_, err = os.Stat(filepath.Join(tmpDir, manifestFolder, summaryFileName))
			Expect(err).To(BeNil())
			_, err = os.Stat(filepath.Join(tmpDir, manifestFolder, "CoolApp-app.yml"))
			Expect(err).To(BeNil())
		})
	})
})
