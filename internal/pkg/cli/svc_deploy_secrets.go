// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"sort"
	"strings"

	awssecretsmanager "github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	awsssm "github.com/aws/copilot-cli/internal/pkg/aws/ssm"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

const (
	secretStoreSSM            = "SSM Parameter Store"
	secretStoreSecretsManager = "Secrets Manager"
)

type missingSecret struct {
	name  string
	store string
}

func (o *deploySvcOpts) warnMissingSecrets(ctx context.Context, mft any) {
	for _, secret := range o.missingSecrets(ctx, mft) {
		log.Warningf("%s secret %s does not exist; deployment may fail until it is created.\n",
			secret.store, color.HighlightUserInput(secret.name))
	}
}

func (o *deploySvcOpts) missingSecrets(ctx context.Context, mft any) []missingSecret {
	var missing []missingSecret
	seen := make(map[missingSecret]struct{})
	for _, secret := range workloadSecrets(mft) {
		name, store, ok := secretRef(secret)
		if !ok {
			continue
		}
		ref := missingSecret{name: name, store: store}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		switch store {
		case secretStoreSecretsManager:
			if o.secretsmanager == nil {
				continue
			}
			if _, err := o.secretsmanager.DescribeSecret(name); err != nil {
				var notFound *awssecretsmanager.ErrSecretNotFound
				if errors.As(err, &notFound) {
					missing = append(missing, ref)
				}
			}
		default:
			if o.ssmParamGetter == nil {
				continue
			}
			if _, err := o.ssmParamGetter.GetSecretValue(ctx, name); err != nil {
				var notFound *awsssm.ErrParameterNotFound
				if errors.As(err, &notFound) {
					missing = append(missing, ref)
				}
			}
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].store == missing[j].store {
			return missing[i].name < missing[j].name
		}
		return missing[i].store < missing[j].store
	})
	return missing
}

func secretRef(secret manifest.Secret) (string, string, bool) {
	if secret.RequiresImport() {
		return "", "", false
	}
	name := secret.Value()
	if name == "" {
		return "", "", false
	}
	if secret.IsSecretsManagerName() || strings.Contains(name, ":secretsmanager:") {
		return name, secretStoreSecretsManager, true
	}
	return name, secretStoreSSM, true
}

func workloadSecrets(mft any) []manifest.Secret {
	switch mft := mft.(type) {
	case *manifest.LoadBalancedWebService:
		return ecsWorkloadSecrets(mft.TaskConfig, mft.Logging, mft.Sidecars)
	case *manifest.BackendService:
		return ecsWorkloadSecrets(mft.TaskConfig, mft.Logging, mft.Sidecars)
	case *manifest.WorkerService:
		return ecsWorkloadSecrets(mft.TaskConfig, mft.Logging, mft.Sidecars)
	case *manifest.RequestDrivenWebService:
		return appendSecrets(nil, mft.Secrets)
	default:
		return nil
	}
}

func ecsWorkloadSecrets(task manifest.TaskConfig, logging manifest.Logging, sidecars map[string]*manifest.SidecarConfig) []manifest.Secret {
	secrets := appendSecrets(nil, task.Secrets)
	secrets = appendSecrets(secrets, logging.Secrets)
	secrets = appendSecrets(secrets, logging.SecretOptions)
	for _, sidecar := range sidecars {
		if sidecar == nil {
			continue
		}
		secrets = appendSecrets(secrets, sidecar.Secrets)
	}
	return secrets
}

func appendSecrets(dst []manifest.Secret, src map[string]manifest.Secret) []manifest.Secret {
	for _, secret := range src {
		dst = append(dst, secret)
	}
	return dst
}
