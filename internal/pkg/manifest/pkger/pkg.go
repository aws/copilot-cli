// Package pkger provides functionality to transform an application manifest
// into structures that can be consumed by infrastructure-as-code providers.
package pkger

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	deploycfn "github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

type projectDecriber interface {
	GetProjectResourcesByRegion(project *archer.Project, region string) (*archer.ProjectRegionalResources, error)
}

type WebAppPkger struct {
	app      *manifest.LBFargateManifest
	env      *archer.Environment
	project  *archer.Project
	imageTag string

	projectDecriber projectDecriber
}

func NewWebAppPkger(app *manifest.LBFargateManifest, env *archer.Environment, project *archer.Project, imageTag string) (*WebAppPkger, error) {
	sess, err := session.NewProvider().Default()
	if err != nil {
		return nil, fmt.Errorf("new default session: %w", err)
	}
	return &WebAppPkger{
		app:             app,
		env:             env,
		project:         project,
		projectDecriber: deploycfn.New(sess),
	}, nil
}

func (p *WebAppPkger) Stack() (*cloudformation.Stack, error) {
	resources, err := p.projectDecriber.GetProjectResourcesByRegion(p.project, p.env.Region)
	if err != nil {
		return nil, err
	}

	repoURL, ok := resources.RepositoryURLs[p.app.Name]
	if !ok {
		return nil, &ErrRepoNotFound{
			appName:       p.app.Name,
			envRegion:     p.env.Region,
			projAccountID: p.project.AccountID,
		}
	}

	in := &deploy.CreateLBFargateAppInput{
		App:            p.app,
		Env:            p.env,
		ImageTag:       p.imageTag,
		AdditionalTags: p.project.Tags,
		ImageRepoURL:   repoURL,
	}
	conf, err := stack.NewLBFargateStack(in)
	if p.project.RequiresDNSDelegation() {
		conf, err = stack.NewHTTPSLBFargateStack(in)
	}
	if err != nil {
		return nil, fmt.Errorf("create stack configuration: %w", err)
	}

	tpl, err := conf.Template()
	if err != nil {
		return nil, err
	}
	stack := cloudformation.NewStack(conf.StackName(), tpl)
	stack.Parameters = conf.Parameters()
	stack.Tags = conf.Tags()
	return stack, nil
}

type ErrRepoNotFound struct {
	appName       string
	envRegion     string
	projAccountID string
}

func (e *ErrRepoNotFound) Error() string {
	return fmt.Sprintf("ECR repository not found for application %s in region %s and account %s", e.appName, e.envRegion, e.projAccountID)
}
