// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy a workload.
package deploy

import (
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/mod/semver"

	sdkcfn "github.com/aws/aws-sdk-go/service/cloudformation"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

type uploader interface {
	Upload(bucket, key string, data io.Reader) (string, error)
}

type stackSerializer interface {
	templater
	SerializedParameters() (string, error)
	Parameters() ([]*sdkcfn.Parameter, error)
	Tags() []*sdkcfn.Tag
}

type versionGetter interface {
	Version() (string, error)
}

type serviceForceUpdater interface {
	ForceUpdateService(app, env, svc string) error
	LastUpdatedAt(app, env, svc string) (time.Time, error)
}

type aliasCertValidator interface {
	ValidateCertAliases(aliases []string, certs []string) error
}

type svcDeployer struct {
	*workloadDeployer
	svcUpdater serviceForceUpdater
	now        func() time.Time
}

func newSvcDeployer(in *WorkloadDeployerInput) (*svcDeployer, error) {
	wkldDeployer, err := newWorkloadDeployer(in)
	if err != nil {
		return nil, err
	}
	return &svcDeployer{
		workloadDeployer: wkldDeployer,
		now:              time.Now,
	}, nil
}

func (d *svcDeployer) Deploy(stack Stack, deployOpts DeployOpts) error {
	opts := []awscloudformation.StackOption{
		awscloudformation.WithRoleARN(d.env.ExecutionRoleARN),
	}
	if deployOpts.DisableRollback {
		opts = append(opts, awscloudformation.WithDisableRollback())
	}
	cmdRunAt := d.now()
	if err := d.deployer.DeployService(stack, d.resources.S3Bucket, opts...); err != nil {
		var errEmptyCS *awscloudformation.ErrChangeSetEmpty
		if !errors.As(err, &errEmptyCS) {
			return fmt.Errorf("deploy service: %w", err)
		}
		if !deployOpts.ForceNewUpdate {
			log.Warningln("Set --force to force an update for the service.")
			return fmt.Errorf("deploy service: %w", err)
		}
	}
	// Force update the service if --force is set and the service is not updated by the CFN.
	if deployOpts.ForceNewUpdate {
		lastUpdatedAt, err := d.svcUpdater.LastUpdatedAt(d.app.Name, d.env.Name, d.name)
		if err != nil {
			return fmt.Errorf("get the last updated deployment time for %s: %w", d.name, err)
		}
		if cmdRunAt.After(lastUpdatedAt) {
			if err := d.forceDeploy(&forceDeployInput{
				spinner:    d.spinner,
				svcUpdater: d.svcUpdater,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAppVersionForAlias(appName string, appVersionGetter versionGetter) error {
	appVersion, err := appVersionGetter.Version()
	if err != nil {
		return fmt.Errorf("get version for app %s: %w", appName, err)
	}
	diff := semver.Compare(appVersion, deploy.AliasLeastAppTemplateVersion)
	if diff < 0 {
		return fmt.Errorf(`alias is not compatible with application versions below %s`, deploy.AliasLeastAppTemplateVersion)
	}
	return nil
}

func logAppVersionOutdatedError(name string) {
	log.Errorf(`Cannot deploy service %s because the application version is incompatible.
To upgrade the application, please run %s first (see https://aws.github.io/copilot-cli/docs/credentials/#application-credentials).
`, name, color.HighlightCode("copilot app upgrade"))
}
