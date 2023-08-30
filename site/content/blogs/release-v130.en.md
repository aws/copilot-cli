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

## Ctrl-C

## Deployment actions
Maybe you want to run a database migration before a workload deployment, and the workload's health check depends on the
update. Or maybe you'd like your pipeline to execute some end-to-end or integration tests after a workload deployment. These actions are
now possible with [Copilot pipelines](../docs/concepts/pipelines.en.md)!  

While Copilot has supported ['test_commands'](https://aws.github.io/copilot-cli/docs/manifest/pipeline/#stages-test-cmds) for some time now, [pre-](https://aws.github.io/copilot-cli/docs/manifest/pipeline/#stages-predeployments) and [post-deployment](https://aws.github.io/copilot-cli/docs/manifest/pipeline/#stages-postdeployments) actions extend pipeline functionality 
and flexibility. For `test_commands`, Copilot inlines a buildspec with your command strings into the pipeline
Cloudformation template; for `pre_deployments` and `post_deployments`, Copilot reads the [buildspec(s)](https://docs.aws.amazon.com/codebuild/latest/userguide/build-spec-ref.html)
in your local workspace.  

You control all the configuration for these actions right in your [pipeline manifest](../docs/manifest/pipeline.en.md). You may have multiple pre-deployment actions and multiple 
post-deployment actions; for each one, you must provide the path to the [buildspec](https://docs.aws.amazon.com/codebuild/latest/userguide/build-spec-ref.html),
relative to your project root, in the `[pre_/post_]deployments.buildspec` field. Copilot will generate a CodeBuild project for your action, deployed
in the same region as the pipeline and app. Use Copilot commands–such as `copilot svc exec` or `copilot task run`–within your buildspec to access the VPC of the environment
being deployed or deployed to; you may use the `$COPILOT_APPLICATION_NAME` and `$COPILOT_ENVIRONMENT_NAME` Copilot environment variables
to reuse your buildspec file for multiple environments.

You can even specify the order of actions within your pre-deployment and
post-deployment groups by using the `depends_on` field; by default, the actions will run in parallel. 

`post_deployments` and `test_commands` are mutually exclusive.
```yaml
stages:
  - name: test
    require_approval: true
    pre_deployments:
      db_migration: # The name of this action.
        buildspec: copilot/pipelines/demo-api-frontend-main/buildspecs/buildspec.yml # The path to the buildspec.
    deployments: # Optional, ordering of deployments. 
      orders:
      warehouse:
      frontend:
        depends_on: [orders, warehouse]
    post_deployments:
      db_migration:
        buildspec: copilot/pipelines/demo-api-frontend-main/buildspecs/post_buildspec.yml
      integration:
        buildspec: copilot/pipelines/demo-api-frontend-main/buildspecs/integ-buildspec.yml
        depends_on: [db_migration] # Optional, ordering of actions.
```

## `copilot deploy` enhancements

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.30.0)