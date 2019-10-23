package cli

import "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"

type identityService interface {
	Get() (identity.Caller, error)
}
