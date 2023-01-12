---
title: 'AWS Copilot v1.25: Environment addons and static content delivery.'
twitter_title: 'AWS Copilot v1.25'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.25: Environment addons and static content delivery.

Posted On: Jan 17, 2023

The AWS Copilot core team is announcing the Copilot v1.25 release.  
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has almost 400 people online and over 2.6k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.25 brings several new features and improvements:

- **Environment Addons**: [See detailed section](#environment-addons).
- **Static Content Delivery With CloudFront**: [See detailed section](#static-content-delivery-with-cloudfront).

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Environment Addons
### Import Values From CloudFormation Stacks In Workload Manifests

You can now import values from environment addons' CloudFormation stacks or any other stack in your workload manifest using `from_cfn`.
To reference a value from another CloudFormation stack, users should first export the output value from the source stack.

Here is an example on how the `Outputs` section of a CloudFormation template looks when exporting values from other stacks or creating cross-stack references.

```yaml
Outputs:
  WebBucketURL:
    Description: URL for the website bucket
    Value: !GetAtt WebBucket.WebsiteURL
    Export:
      Name: stack-WebsiteUrl # <- Unique export name within the region.
```

To find our more, see [this page](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html).

For now, `from_cfn` is added only to the following workload manifest fields.

```yaml
variables:
  LOG_LEVEL: info
  WebsiteUrl:
    from_cfn: stack-WebsiteUrl
```

```yaml
secrets:
  GIT_USERNAME:
    from_cfn: stack-SSMGHUserName
```

```yaml
logging:
  secretOptions:
    GIT_USERNAME:
      from_cfn: stack-SSMGHUserName
```

```yaml
sidecars:
  secrets:
    GIT_USERNAME:
      from_cfn: stack-SSMGHUserName
```

```yaml
network:
  vpn:
    security_groups:
      - sg-1234
      - from_cfn: UserDBAccessSecurityGroup
```

## Static Content Delivery With CloudFront

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0)