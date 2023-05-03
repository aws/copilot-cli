package cleantest

import "errors"

type Succeeds struct{}

func (*Succeeds) Clean(_, _, _ string) error {
	return nil
}

type Fails struct{}

func (*Fails) Clean(_, _, _ string) error {
	return errors.New("an error")
}
