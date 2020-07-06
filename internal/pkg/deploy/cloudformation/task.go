package cloudformation

import (
	"errors"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

// DeployTask deploys a task stack and waits until the deployment is done.
// If the task stack doesn't exist, then it creates the stack.
// If the task stack already exists, it updates the stack.
func (cf CloudFormation) DeployTask(input *deploy.CreateTaskResourcesInput) error {
	conf := stack.NewTaskStackConfig(input)
	stack, err := toStack(conf)
	if err != nil {
		return err
	}

	if err := cf.cfnClient.CreateAndWait(stack); err != nil {
		var errAlreadyExists *cloudformation.ErrStackAlreadyExists
		if !errors.As(err, &errAlreadyExists) {
			return err
		}
		return cf.cfnClient.UpdateAndWait(stack)
	}
	return nil
}
