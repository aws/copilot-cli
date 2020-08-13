package task

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/e2e/internal/client"
	"github.com/aws/copilot-cli/e2e/internal/command"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var appName, envName, groupName, taskStackName, repoName string

/**
The task suite runs through several tests focusing on running one-off tasks with different configurations.
*/
func TestTask(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Task Suite")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())

	appName = fmt.Sprintf("e2e-task-%d", time.Now().Unix())
	envName = "test"
	groupName = fmt.Sprintf("e2e-task-%d", time.Now().Unix())
	// We name task stack in format of "task-${groupName}".
	// See https://github.com/aws/copilot-cli/blob/e9e3114561e740c367fb83b5e075750f232ad639/internal/pkg/deploy/cloudformation/stack/name.go#L26.
	taskStackName = fmt.Sprintf("task-%s", groupName)
	// We name ECR repo name in format of "copilot-${groupName}".
	// See https://github.com/aws/copilot-cli/blob/e9e3114561e740c367fb83b5e075750f232ad639/templates/task/cf.yml#L75.
	repoName = fmt.Sprintf("copilot-%s", groupName)
})

var _ = AfterSuite(func() {
	// Clean ECR repo before deleting the stack.
	err := command.Run("aws", []string{"ecr", "delete-repository", "--repository-name", repoName, "--force"})
	Expect(err).NotTo(HaveOccurred(), "delete ecr repo")
	// Delete task stack.
	err = command.Run("aws", []string{"cloudformation", "delete-stack", "--stack-name", taskStackName})
	Expect(err).NotTo(HaveOccurred(), "start deleting task stack")
	// Wait until task stack is removed.
	err = command.Run("aws", []string{"cloudformation", "wait", "stack-delete-complete", "--stack-name", taskStackName})
	Expect(err).NotTo(HaveOccurred(), "task stack delete complete")
	// Delete Copilot application.
	_, err = cli.AppDelete(map[string]string{"test": "default"})
	Expect(err).NotTo(HaveOccurred(), "delete Copilot application")
})

func BeforeAll(fn func()) {
	first := true
	BeforeEach(func() {
		if first {
			fn()
			first = false
		}
	})
}
