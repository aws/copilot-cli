---
title: 'AWS Copilot v1.29: Pipeline template overrides and CloudFront cache invalidation'
twitter_title: 'AWS Copilot v1.29'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.29: Pipeline template overrides and CloudFront cache invalidation!

Posted On: July 19, 2023

The AWS Copilot core team is announcing the Copilot v1.29 release.

Special thanks to [@tjhorner](https://github.com/tjhorner) and [@build-with-aws-copilot](https://github.com/build-with-aws-copilot), who contributed to this release.
Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) is growing and has nearly 500 people online and over 2.9k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.29 brings big enhancements to help you develop more flexibly and efficiently:

- **Pipeline overrides**: In [v1.27.0](https://aws.github.io/copilot-cli/blogs/release-v127/#extend-copilot-generated-aws-cloudformation-templates), we introduced CDK and YAML patch overrides for workload and environment CloudFormation templates. Now you can enjoy the same extensibility for Copilot pipeline templates! [See detailed section](#pipeline-overrides).
- **Static Site enhancements**: We've improved our [latest workload type](https://aws.github.io/copilot-cli/blogs/release-v128/#static-site-service-type) with CloudFront cache invalidation and Static Site-tailored operational commands. [See detailed section](#static-site-enhancements).

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision Infrastructure as Code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Pipeline overrides
Copilot pipelines are much more nimble and extensible with CDK and YAML path overrides! This feature provides a safe and easy way to modify your pipeline's CloudFormation template.  
Much like the other override commands, you can run `copilot pipeline override` to customize that CloudFormation template, with the option of using either the CDK or YAML.  
A new `--diff` flag for `copilot pipeline deploy` enables you to preview the differences between your last deployed CloudFormation template and any local changes before the deployment is executed. Copilot will confirm that you'd like to proceed; use the `--yes` flag to skip the confirmation: `copilot pipeline deploy --diff --yes`.  

To learn more about overrides and to see examples, check out the [CDK overrides guide](../docs/developing/overrides/cdk.md) and [YAML patch overrides guide](../docs/developing/overrides/yamlpatch.md).

## Static Site enhancements
For more dynamic development, Copilot will now invalidate the CloudFront edge cache each time you redeploy a Static Site workload, enabling you to see and deliver your updated content right away.

Our operational commands have some Static Site-specific additions as well:
`copilot svc show` for Static Site workloads now includes a tree representation of your S3 bucket's contents.

```console
Service name: static-site
About

  Application  my-app
  Name         static-site
  Type         Static Site

Routes
  Environment  URL
  -----------  ---
  test         https://d399t9j1xbplme.cloudfront.net/

S3 Bucket Objects

  Environment  test
.
├── ReadMe.md
├── error.html
├── index.html
├── Images
│   ├── SomeImage.PNG
│   └── AnotherImage.PNG
├── css
│   ├── Style.css
│   ├── all.min.css
│   └── bootstrap.min.css
└── images
     └── bg-masthead.jpg
```

And `copilot svc status` for Static Site workloads includes the S3 bucket's object count and total size.

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.29.0)