---
title: 'AWS Copilot v1.28: Static Site service type and more!'
twitter_title: 'AWS Copilot v1.28'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.28: Static Site service type and more!

Posted On: May 09, 2023

The AWS Copilot core team is announcing the Copilot v1.28 release.
Special thanks to [@interu](https://github.com/interu), [@0xO0O0](https://github.com/0xO0O0), who contributed to this release.
Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) is growing and has over 400 people online and over 2.8k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.28 brings several new features and improvements:

- **Static Site service type**: You can now deploy and host static websites with AWS S3. [See detailed section](#Static-Site-service-type).
- **Config Multiple Container Ports with the `--port` flag**:`Copilot init` and `Copilot svc init` now allows you to configure multiple ports for Load Balanced Service and Backend Service. [See detailed section](#Config-Multiple-Container-Ports-with-the---port-flag).
- **Container Images Parallel Build**: You can now build main container and sidecar container images in parallel. With parallel build, you can reduce the overall time it takes build and push container images to AWS ECR.

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Static Site service type
Copilot's newest workload type, Static Site, provisions and configures everything you need to create a static website hosted by Amazon S3 and fronted by an Amazon CloudFront distribution.

### Static Site Upload Experience
As with other services, begin your Static Site creation with the `copilot init` or `copilot svc init` command. You may use the `--sources` flag to pass in the path(s) (relative to your project root) to your static asset directories and/or files. Alternatively, you may select the directories/files when prompted.

A manifest will be populated and stored in the `copilot/[service name]` folder. There, you may adjust your asset specifications if you'd like. By default, all directories will be uploaded recursively. If that's not what you want, leverage the `exclude` and `reinclude` fields to add filters. The available pattern symbols:

*: Matches everything
?: Matches any single character
[sequence]: Matches any character in sequence
[!sequence]: Matches any character not in sequence

```yaml
# The manifest for the "example" service.
# Read the full specification for the "Static Site" type at:
#  https://aws.github.io/copilot-cli/docs/manifest/static-site/

# Your service name will be used in naming your resources like S3 buckets, etc.
name: example
type: Static Site
files:
  - source: src/someDirectory
    recursive: true

# You can override any of the values defined above by environment.
# environments:
#   test:
#     files:
#       - source: './blob'
#         recursive: true
#         destination: 'assets'
#         exclude: '*'
#         reinclude:
#           - '*.txt'
#           - '*.png'
```
For more on `exclude` and `reinclude` filters, go [here](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3/index.html#use-of-exclude-and-include-filters).

The `copilot deploy` or `copilot svc deploy` command will provision and launch your static website: creating an S3 bucket and uploading your chosen local files to that bucket, adding necessary permissions, generating a CloudFront distribution with the S3 bucket as the origin, etc. Under the hood, your Static Site service will have a CloudFormation stack, just like other Copilot workloads.

## Config multiple container ports with the `--port` flag

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.28.0)