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

- **NLB enhancements**: You can now add security groups to Copilot-managed [network load balancers](../docs/manifest/lb-web-service.en.md#nlb). NLBs also support the UDP protocol.
- **Better task failure logs**: Copilot will show more descriptive information during deployments when tasks fail, allowing better troubleshooting.
- **`copilot deploy` enhancements**: You can now deploy multiple workloads at once, or deploy all local workloads with `--all`.

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision Infrastructure as Code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## NLB enhancements

Copilot brings UDP traffic support with an update to your Network Load Balancer! The protocol your NLB uses is specified by the [nlb.port](https://aws.github.io/copilot-cli/docs/manifest/lb-web-service/#nlb-port) field.
```
nlb:
  port: 8080/udp
```

!!!warning
  To use the new Security Group, your `NetworkLoadBalancer` and `TargetGroup` resources need to be recreated. With v1.31 this will only happen if you specify `udp` protocol. With v1.33 however, Copilot will make this change for all users.

## Better task failure logs

## `copilot deploy` enhancements