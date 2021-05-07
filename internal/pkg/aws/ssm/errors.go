package ssm

import "fmt"

type ErrParameterAlreadyExists struct {
	name string
}

func (e *ErrParameterAlreadyExists) Error() string {
	return fmt.Sprintf("parameter %s already exists", e.name)
}
