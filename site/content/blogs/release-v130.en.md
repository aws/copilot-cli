---
title: 'AWS Copilot v1.30: `copilot run local`, ctrl-c functionality, pre- and post-deployments, `copilot deploy` enhancements'
twitter_title: 'AWS Copilot v1.30'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.30: `copilot run local` command, Ctrl-C functionality, pre- and post-deployments, `copilot deploy` enhancements

Posted On: August 30, 2023

The AWS Copilot core team is announcing the Copilot v1.30 release.

Special thanks to [@Varun359](https://github.com/Varun359), who contributed to this release.
Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) has over 500 people participating, and our GitHub repository has over 3k stars on [GitHub](http://github.com/aws/copilot-cli/) 🚀.
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.30 brings big enhancements to help you develop more flexibly and efficiently:

- **`copilot run local`**: Copilot has a new operational command that enables you to run services locally! [See detailed section](#copilot-run-local).
- **Ctrl-C**: The wait is over! Roll back your CloudFormation deployment right from the terminal, whenever you want. [See detailed section](#ctrl-c).
- **Pre- and post- deployment pipeline actions**: Insert db migrations, integration tests, and/or other actions before or after workload/environment deployments. [See detailed section](#deployment-actions). 
- **`copilot deploy` enhancements**: We've increased the scope and flexibility of this command. [See detailed section](#copilot-deploy-enhancements).

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision Infrastructure as Code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## `copilot run local`
As you're developing changes to your services, `copilot run local` speeds up your iteration loop by enabling you to test changes to your code
without the overhead of a deployment. To get started, you first need to deploy a version of your service by running `copilot svc deploy`.

After you have your service deployed, you can start making modifications to your code. When you're ready to test your changes, run `copilot run local` and
Copilot will do the following for both your primary container and any sidecars:

1. Build or pull the image specified by [`image`](../docs/manifest/lb-web-service#image)
2. Get the values for secrets specified by [`secrets`](../docs/manifest/lb-web-service#secrets)
3. Get credentials for your current IAM user/role
4. Run images from step 1 with [`variables`](../docs/manifest/lb-web-service#variables), secrets from step 2, and IAM credentials from step 3 on your local machine

Logs from your service are streamed to your terminal. When you're finished testing, type Ctrl+C and Copilot will clean up all of running containers before exiting!

## Ctrl-C

## Deployment actions

## `copilot deploy` enhancements

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.30.0)