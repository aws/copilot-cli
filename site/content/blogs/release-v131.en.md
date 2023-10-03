---
title: 'AWS Copilot v1.31: importing certificates for static site, udp traffic,  ordered deploymennts by `copilot deploy`, and ECS task stopped reasons in progress tracker!
twitter_title: 'AWS Copilot v1.30'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.31: importing certificates for static site, udp traffic, ordered deploymennts by `copilot deploy`, and ECS task stopped reasons in progress tracker!

Posted On: Oct 5, 2023

The AWS Copilot core team is announcing the Copilot v1.31 release.

Special thanks to [@tjhorner](https://github.com/tjhorner) and [@build-with-aws-copilot](https://github.com/build-with-aws-copilot), who contributed to this release.
Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) has over 500 people participating, and our GitHub repository has over 3.1k stars on [GitHub](http://github.com/aws/copilot-cli/) 🚀.
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.30 brings big enhancements to help you develop more flexibly and efficiently:

- **Importing certificates for your static site**: Use any domain name for your static site by importing your own certificates!. [See detailed section](#importing-certificates-for-static-site)
- **UDP traffic support**: You can now deploy a Load-Balanced Web Service that accepts UDP traffic. [See detailed section](#udp-traffic-support).
- **See why your ECS tasks fail to stablize from the progress tracker**: Wether it's the lack of permissions to pull images or secrets, or that the application is failing the health check, you can now see the specific reasons why your ECS tasks fail to stablize, all from the progress tracker. [See detailed section](#better-debug-information-from-progress-tracker).
- **Ordered deployments by `copilot deploy`**: You can now use `copilot deploy` to deploy multiple services or jobs, with optional ordering. [See detailed section](#ordered-deployments-by-copilot-deploy). 

There are also several other mini improvements:
- **`copilot [env/svc] init` improvements**: these `init` commands no longer complains if you are initiating an existing service/job/environment already managed by the same workspace. In addition, `copilot env init` will no longer ask you to select an AWS profile if you have not configured any. ([#5242](https://github.com/aws/copilot-cli/pull/5242) and [#5202](https://github.com/aws/copilot-cli/pull/5202))
- **`copilot env delete` automatically empties the public application load balancer access logs**: previously, if [`http.public.access_logs`](https://aws.github.io/copilot-cli/docs/manifest/environment/#http-public-access-logs) is enabled, chances are `copilot env delete` will fail because of the non-empty S3 bucket that stores the access log. Now, Copilot will first try to empty the bucket so that `copilot env delete` runs smoothly. ([#5248](https://github.com/aws/copilot-cli/pull/5184))
- **Enable versioning on S3 buckets**: Copilot now enables versioning on all of the S3 buckets created by Copilot. ([#5289](https://github.com/aws/copilot-cli/pull/5289))

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision Infrastructure as Code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Importing certificates for static site

## UDP traffic support

## Better debug information from progress tracker

## Ordered deployments by `copilot deploy`

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.31.0)