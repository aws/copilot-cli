// package delete contains common functions for deleting resources
// created through copilot.
package delete

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/config"
)

type imageRemover interface {
	ClearRepository(repoName string) error
}

type regionalSessionProvider interface {
	DefaultWithRegion(region string) (*session.Session, error)
}

type ECREmptier struct {
	AppName         string
	WorkloadName    string
	Environments    []*config.Environment
	SessionProvider regionalSessionProvider

	newImageRemover func(*session.Session) imageRemover // for testing
}

func (e *ECREmptier) defaultNewImageRemover(sess *session.Session) imageRemover {
	return ecr.New(sess)
}

func (e *ECREmptier) EmptyRepos() error {
	if e.newImageRemover == nil {
		e.newImageRemover = e.defaultNewImageRemover
	}

	regions := make(map[string]struct{})
	for _, env := range e.Environments {
		regions[env.Region] = struct{}{}
	}

	repoName := fmt.Sprintf("%s/%s", e.AppName, e.WorkloadName)
	for region := range regions {
		sess, err := e.SessionProvider.DefaultWithRegion(region)
		if err != nil {
			return err
		}

		client := e.newImageRemover(sess)
		if err := client.ClearRepository(repoName); err != nil {
			return err
		}
	}

	return nil
}
