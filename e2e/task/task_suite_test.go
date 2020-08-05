package task

import (
	"fmt"
	"github.com/aws/copilot-cli/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
	"time"
)

var cli *client.CLI
var appName, envName, groupName string

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
})

var _ = AfterSuite(func() {
	//_, err := cli.AppDelete(map[string]string{"test": "default"})
	//Expect(err).NotTo(HaveOccurred())
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