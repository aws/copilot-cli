---
title: 'AWS Copilot v1.33: run local `--use-task-role`, and run local `depends_on` support'
twitter_title: 'AWS Copilot v1.33'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.33: run local `--use-task-role`, and run local `depends_on` support

Posted On: January 8, 2024

The AWS Copilot core team is announcing the Copilot v1.33 release.

Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) has over 500 people participating, and our GitHub repository has over 3,100 stars on [GitHub](http://github.com/aws/copilot-cli/) 🚀.
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.33 brings big enhancements to help you develop more flexibly and efficiently:

- **run local `--use-task-role`**: Elevate your local testing experience with the ECS Task Role using the new `--use-task-role` flag. [See detailed section](#use-ecs-task-role-for-copilot-run-local)
- **run local `depends_on` support**:  Local run containers now respects `depends_on` in your service manifests. [See detailed section](#container-dependencies-support-for-copilot-run-local)

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision Infrastructure as Code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Use ECS Task Role for `copilot run local`

## Container dependencies support for `copilot run local`

`copilot run local` now respects the [depends_on](../docs/manifest/lb-web-service.md#image-depends-on) specified in the service manifest.

For Example:

```
image:
  build: ./Dockerfile
  depends_on:
    nginx: start

nginx:
  image:
    build: ./web/Dockerfile
    essential: true
    depends_on:
      startup: success
```

This means that your main container will start only after nginx sidecar container has started and nginx will start only after startup container is completed successfully.