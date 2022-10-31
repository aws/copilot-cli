---
title: 'AWS Copilot v1.23: App Runner private service, Aurora Serverless v2 and more!'
twitter_title: 'AWS Copilot v1.23'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.23: App Runner private service, Aurora Serverless v2 and more!

Posted On: Oct 31, 2022

The AWS Copilot core team is announcing the Copilot v1.23 release.   
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has over 300 people online and nearly 2.5k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.23 brings several new features and improvements:

- **App Runner private services**: [See detailed section](#app-runner-private-services).
- **Support Aurora Serverless v2 in `storage init`**: [See detailed section](#support-aurora-serverless-v2-in-storage-init).
- **Move misplaced `http` fields in environment manifest (backward-compatible!):** [See detailed section](#move-misplaced-http-fields-in-environment-manifest-backward-compatible).
- **Restrict container access to root file system to read-only:** [See manifest field](https://aws.github.io/copilot-cli/docs/manifest/lb-web-service/#storage-readonlyfs) [(#4062)](https://github.com/aws/copilot-cli/pull/4062).
- **Configure SSL policy for your ALB’s HTTPS listener:** [See manifest field](https://aws.github.io/copilot-cli/docs/manifest/environment/#http-public-sslpolicy) [(#4099)](https://github.com/aws/copilot-cli/pull/4099).
- **Restrict ingress to your ALB through source IPs**: [See manifest field](https://aws.github.io/copilot-cli/docs/manifest/environment/#http-public-ingress-source-ips) [(#4103)](https://github.com/aws/copilot-cli/pull/4103).


???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## App Runner private services

## Support Aurora Serverless v2 in `storage init`

## Move misplaced `http` fields in environment manifest (backward-compatible!)

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.23.0)
