package describe

//import (
//	"fmt"
//	"testing"
//
//	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
//	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
//)
//
//func TestJobDescriber_TaskDefinition(t *testing.T) {
//	sess, err := sessions.NewProvider().Default()
//	if err != nil {
//		fmt.Println(err)
//		return
//	}
//
//	j := &JobDescriber{
//		App:       "final-test",
//		Env:       "test",
//		Job:       "expr-job",
//		ECSClient: ecs.New(sess),
//	}
//
//	td, err := j.TaskDefinition()
//	fmt.Println(err)
//	fmt.Println(td)
//	fmt.Println(td.ExecutionRole)
//}
