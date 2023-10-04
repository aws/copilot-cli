---
title: 'AWS Copilot v1.31: NLB enhancements, better task failure logs, `copilot deploy` enhancements
twitter_title: 'AWS Copilot v1.31'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# 'AWS Copilot v1.31: NLB enhancements, better task failure logs, `copilot deploy` enhancements

Posted On: October 5, 2023

The AWS Copilot core team is announcing the Copilot v1.31 release.

Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) has over 500 people participating, and our GitHub repository has over 3k stars on [GitHub](http://github.com/aws/copilot-cli/) 🚀.
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.31 brings big enhancements to help you develop more flexibly and efficiently:

- **NLB enhancements**: You can now add security groups to Copilot-managed NLBs. NLBs also support the UDP protocol.
- **Better task failure logs**: Copilot will show more descriptive information during deployments when tasks fail, allowing better troubleshooting.
- **`copilot deploy` enhancements: You can now deploy multiple workloads at once, or deploy all local workloads with `--all`.

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision Infrastructure as Code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## NLB enhancements

## Better task failure logs

## `copilot deploy` enhancements
`copilot deploy` now supports deploying multiple workloads with one command. You can specify multiple workloads with the
`--name` flag, use the new `--all` flag in conjunction with `--init-wkld` to initialize and deploy all local workloads,
and you can now provide a "deployment order" tag when specifying service names. 

For example, if you have cloned a new repository which includes multiple workloads, you can initialize the environment and 
all services with one command.
```console
copilot deploy --init-env --deploy-env -e dev --all --init-wkld
```

If you have a service which must be deployed before another--for example, there is worker service which subscribes to a topic exposed
by a different service in the workspace--you can specify names and orders with `--all`.
```console
copilot deploy --all -n fe/1 -n worker/2
```
This will deploy `fe`, then `worker`, then the remaining services or jobs in the workspace.