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
- **Roll back deployments with Ctrl-C**: The wait is over! Roll back your CloudFormation deployment right from the terminal, whenever you want. [See detailed section](#roll-back-deployments-with-ctrl-c).
- **Pre- and post- deployment pipeline actions**: Insert db migrations, integration tests, and/or other actions before or after workload/environment deployments. [See detailed section](#deployment-actions). 
- **`copilot deploy` enhancements**: We've increased the scope and flexibility of this command. [See detailed section](#copilot-deploy-enhancements).
- **`--detach flag`**: Skip progress of CloudFormation stack events on your terminal. [see detailed section](#use---detach-to-deploy-without-waiting).

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

1. Build or pull the image specified by [`image`](../docs/manifest/lb-web-service.en.md#image)
2. Get the values for secrets specified by [`secrets`](../docs/manifest/lb-web-service.en.md#secrets)
3. Get credentials for your current IAM user/role
4. Run images from step 1 with [`variables`](../docs/manifest/lb-web-service.en.md#variables), secrets from step 2, and IAM credentials from step 3 on your local machine

Logs from your service are streamed to your terminal. When you're finished testing, type Ctrl-C and Copilot will clean up all of running containers before exiting!

## Roll back deployments with Ctrl-C

While waiting for your service, job, or environment to deploy, you can now hit `Ctrl-C` to cancel the update. This will either rollback your stack to its previous configuration, or delete the stack if it was being deployed for the first time.

If you hit 'Ctrl-C' a second time, the program will exit, but the stack rollback/deletion will continue.

`Ctrl-C` is now enabled for `copilot svc deploy`, `copilot job deploy`, `copilot env deploy` and `copilot deploy` commands.

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
`copilot deploy` now supports initialization of workloads, and initialization and deployment of environments. 
You can now start from a repo containing only an app and manifests, and with a single command, deploy a working 
environment and service. You can also deploy environments before deploying your desired workload. 

For example, imagine you would like to clone and deploy a repository containing manifests for a "prod" environment and "frontend" and "backend" services.
`copilot deploy` will now prompt you to initialize your workload and environment if necessary, ask you for credentials 
for the account in which you'd like to deploy the environment, then deploy it and the workload. 
```console
$ git clone myrepo
$ cd myrepo
$ copilot app init myapp
$ copilot deploy -n frontend -e prod
```

To specify the profile and region where the environment will be deployed:
```console
$ copilot deploy --region us-west-2 --profile prod-profile -e prod --init-env
```

To skip deploying the environment if it already exists: 
```console
$ copilot deploy --deploy-env=false 
```

## Use `--detach` to deploy without waiting

Typically, after you run any `deploy` commands, Copilot prints the progress to your terminal and waits for the deployment to finish.

Now, if you don't want Copilot to wait, you can use the `--detach` flag. Copilot will trigger the deployment and exit the program, without printing the progress or waiting for the deployment.

The `--detach` flag is available for `copilot svc deploy`, `copilot job deploy`, `copilot env deploy` and `copilot deploy` commands.

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.30.0)