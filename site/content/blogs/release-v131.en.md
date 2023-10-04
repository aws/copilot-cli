---
title: 'AWS Copilot v1.31: NLB enhancements, better task failure logs, `copilot deploy` enhancements'
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
- **Importing an ACM certificate for your Static Site**: You can now bring your own ACM certificate for the Static Site service. [See detailed section](#importing-an-acm-certificate-for-your-static-site)

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

## Importing an ACM certificate for your Static Site

Copilot now introduces a new field `http.certificate` in the [Static Site manifest](../docs/manifest/static-site.en.md). You can specify with the ARN of any validated ACM certificate in `us-east-1` as below:

```yaml
http:
  alias: example.com
  certificate: "arn:aws:acm:us-east-1:1234567890:certificate/e5a6e114-b022-45b1-9339-38fbfd6db3e2"
```

Note that `example.com` must be the domain name or any subject alternative name of the certificate you are bringing in, and we'll use the imported certificate for your HTTPS traffic instead of creating a Copilot-managed one.