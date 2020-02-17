package multi_app_project_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/e2e/internal/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cli *client.CLI
var projectName string

/**
The multi app suite runs through several tests focusing on creating multiple
apps in one project.
*/
func TestMultiAppProject(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Multiple App Suite (one workspace)")
}

var _ = BeforeSuite(func() {
	ecsCli, err := client.NewCLI()
	cli = ecsCli
	Expect(err).NotTo(HaveOccurred())
	projectName = fmt.Sprintf("e2e-multiapp-%d", time.Now().Unix())
})

var _ = AfterSuite(func() {
	_, err := cli.ProjectDelete(map[string]string{"test": "default"})
	Expect(err).NotTo(HaveOccurred())
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
