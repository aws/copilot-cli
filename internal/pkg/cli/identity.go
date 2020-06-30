package cli

import "github.com/aws/copilot-cli/internal/pkg/aws/identity"

type identityService interface {
	Get() (identity.Caller, error)
}
