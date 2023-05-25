---
title: 'AWS Copilot v1.28: Static Site service type is here!'
twitter_title: 'AWS Copilot v1.28'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.28: Static Site service type is here!

Posted On: May 24, 2023

The AWS Copilot core team is announcing the Copilot v1.28 release.
Special thanks to [@interu](https://github.com/interu), [@0xO0O0](https://github.com/0xO0O0), [@andreas-bergstrom](https://github.com/andreas-bergstrom) who contributed to this release.
Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) is growing and has over 400 people online and over 2.9k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.28 brings several new features and improvements:

- **Static Site service type**: You can now deploy and host static websites with AWS S3. [See detailed section](#static-site-service-type).
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

Let's say you want to launch a straightforward, read-only website. You don't need a backend or a database, you don't need to personalize the site based on the user or store any information. Make a static site! This workload type is relatively simple and quick to launch, and is highly performant. 

### Static Site Upload Experience
After you have created your static assets (HTML file and any CSS and/or JavaScript, etc. files), begin your Static Site creation with the [`copilot init`](../docs/commands/init.en.md) command, or [`copilot svc init`](../docs/commands/svc-init.en.md) if you've already run `copilot app init` and `copilot env init`. You may use the `--sources` flag to pass in the path(s) (relative to your project root) to your static asset directories and/or files. Alternatively, you may select the directories/files when prompted.

A manifest will be populated and stored in the `copilot/[service name]` folder. There, you may adjust your asset specifications if you'd like. By default, all directories will be uploaded recursively. If that's not what you want, leverage the `exclude` and `reinclude` fields to add filters. The available pattern symbols:  

`*`: Matches everything  
`?`: Matches any single character  
`[sequence]`: Matches any character in sequence  
`[!sequence]`: Matches any character not in sequence  

```yaml
# The manifest for the "example" service.
# Read the full specification for the "Static Site" type at:
#  https://aws.github.io/copilot-cli/docs/manifest/static-site/

# Your service name will be used in naming your resources like S3 buckets, etc.
name: example
type: Static Site

http:
  alias: 'example.com'

files:
  - source: src/someDirectory
    recursive: true
  - source: someFile.html

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

The [`copilot deploy`](../docs/commands/deploy.en.md) or [`copilot svc deploy`](../docs/commands/svc-deploy.en.md) command will provision and launch your static website: creating an S3 bucket and uploading your chosen local files to that bucket, and generating a CloudFront distribution with the S3 bucket as the origin. Under the hood, your Static Site service will have a CloudFormation stack, just like other Copilot workloads.

!!! note
    [Server access logging](https://docs.aws.amazon.com/AmazonS3/latest/userguide/ServerLogs.html) for the Static Site S3 bucket is not enabled by default, because object uploading is managed by Copilot.

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.28.0)