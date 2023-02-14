---
title: 'AWS Copilot v1.26: Automate rollbacks with CloudWatch alarms, build sidecar images, and `storage init` for env addons'
twitter_title: 'AWS Copilot v1.26'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.26: Automate rollbacks with CloudWatch alarms, build sidecar images, and `storage init` for env addons

Posted On: Feb 20, 2023

The AWS Copilot core team is announcing the Copilot v1.26 release.  
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has almost 400 people online and over 2.6k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.26 brings several new features and improvements:

- **Service alarm-based rollback**: [See detailed section](#service-alarm-based-rollback).
- **`storage init` for environment addons**: [See detailed section](#storage-init-for-environment-addons).
- **Sidecar image build**: [See detailed section](#sidecar-image-build).
- **Request-Driven Web Service secrets support**: [See detailed section](#request-driven-web-service-secrets-support).

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Service alarm-based rollback

## `storage init` for environment addons

## Sidecar image build

## Request-Driven Web Service secrets support
You can now add your secrets(SSM Parameter Store or AWS Secrets Manager) to your App Runner service as environment variables using Copilot. First ensure that your secrets are tagged correctly. Then simply update your Request-Driven Web Service manifest with:
```yaml
  secrets:
    secret-name-as-env-variable: secret-name-from-aws
```
And deploy! Your service can now access the secret as an environment variable.
For more detailed use of the secrets field:
```yaml
secrets:
  # SecretsManager
  # (Recommended) Option 1. Referring to the secret by name.
  DB:
    secretsmanager: 'demo/test/mysql'
  # You can refer to a specific key in the JSON blob.
  DB_PASSWORD:
    secretsmanager: 'demo/test/mysql:password::'
  # You can substitute predefined environment variables to keep your manifest succinct.
  DB_PASSWORD:
        secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql:password::'

  # Option 2. Alternatively, you can refer to the secret by ARN.
  DB: 'arn:aws:secretsmanager:us-west-2:111122223333:secret:demo/test/mysql-Yi6mvL'
 
  #SSM Parameter Store
  # (Recommended) Option 1. Referring to the secret by ARN.
  GITHUB_WEBHOOK_SECRET: 'arn:aws:ssm:us-east-1:615525334900:parameter/GH_WEBHOOK_SECRET'

  # Option 2. Referring to the secret by name.
  GITHUB_WEBHOOK_SECRET: GITHUB_WEBHOOK_SECRET
```
For more details see (../../docs/manifest/rd-web-service/#secrets-from-cfn).

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0)
