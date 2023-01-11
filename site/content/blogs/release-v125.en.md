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

## Static Content Delivery With CloudFront
You can now bring your own S3 bucket to work with CloudFront for faster static content delivery. More native support for bucket management (for example, bucket creation and assets upload) will be included in future releases.

### (Optional) Create an S3 bucket
If you don't have an existing S3 bucket, use either S3 console/AWS CLI/SDK to create an S3 bucket. Note that for security concern, we strongly recommend to create a private S3 bucket which blocks public access by default.

### Configuring CloudFront in env manifest
You can use CloudFront with an S3 bucket as the origin by configuring the environment manifest as below:

```yaml
cdn:
  static_assets:
    location: cf-s3-ecs-demo-bucket.s3.us-west-2.amazonaws.com
    alias: example.com
    path: static/*
```

More specifically `location` is the [DNS domain name of the S3 bucket](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/distribution-web-values-specify.html#DownloadDistValuesDomainName). And the static assets will be accessible at `example.com/static/*`.

### (Optional) Update bucket policy
If the bucket you use for CloudFront is **private**, you need to update the bucket policy to grant read access to CloudFront. To use the example above, we need to update the bucket policy for `cf-s3-ecs-demo-bucket` to

```json
{
    "Version": "2012-10-17",
    "Statement": {
        "Sid": "AllowCloudFrontServicePrincipalReadOnly",
        "Effect": "Allow",
        "Principal": {
            "Service": "cloudfront.amazonaws.com"
        },
        "Action": "s3:GetObject",
        "Resource": "arn:aws:s3:::cf-s3-ecs-demo-bucket/*",
        "Condition": {
            "StringEquals": {
                "AWS:SourceArn": "arn:aws:cloudfront::111122223333:distribution/EDFDVBD6EXAMPLE"
            }
        }
    }
}
```

Note that you can found the CloudFront distribution ID by running `copilot env show --resources`.

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0)
