package ssm

import "fmt"

// ErrParameterAlreadyExists occurs when the parameter with name already existed.
type ErrParameterAlreadyExists struct {
	name string
}

func (e *ErrParameterAlreadyExists) Error() string {
	return fmt.Sprintf("parameter %s already exists", e.name)
}
