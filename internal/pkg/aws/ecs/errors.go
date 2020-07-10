package ecs

import "fmt"

// ErrNoDefaultCluster occurs when the default cluster is not found.
type ErrNoDefaultCluster struct {}

func (e *ErrNoDefaultCluster) Error() string {
    return fmt.Sprintf("no default cluster is found")
}

