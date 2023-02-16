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
- **Request-Driven Web Service secret support**: [See detailed section](#request-driven-web-service-secret-support).

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
You can now use Copilot to build sidecar container images natively from a Dockerfile, and get them pushed to ECR.
You can modify your workload manifest as below.

You can just specify the path to your Dockerfile as string.

```yaml
sidecars:
  nginx:
    image:
      build: path/to/dockerfile
```

You can also specify build as a map.

```yaml
sidecars:
  nginx:
    image:
      build:
        dockerfile: path/to/dockerfile
        context: context/dir
        target: build-stage
        cache_from:
          - image: tag
        args: value
```

You can specify an existing image name instead of building from a Dockerfile.

```yaml
sidecars:
  nginx:
    image: 123457839156.dkr.ecr.us-west-2.amazonaws.com/demo/front:nginx-latest
```
Or you can provide the image name using location field.

```yaml
sidecars:
  nginx:
    image:
      location:  123457839156.dkr.ecr.us-west-2.amazonaws.com/demo/front:nginx-latest
```

## Request-Driven Web Service secret support

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0)
