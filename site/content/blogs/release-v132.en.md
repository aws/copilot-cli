---
title: 'AWS Copilot v1.32: `run local --proxy`, `run local --watch`, imported ALB support
twitter_title: 'AWS Copilot v1.32'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# 'AWS Copilot v1.32: `run local --proxy`, `run local --watch`, imported ALB support

Posted On: November 9, 2023

The AWS Copilot core team is announcing the Copilot v1.32 release.

Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) has over 500 people participating, and our GitHub repository has over 3,100 stars on [GitHub](http://github.com/aws/copilot-cli/) 🚀.
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.32 brings big enhancements to help you develop more flexibly and efficiently:

- **`copilot run local --proxy`**: Proxy outbound traffic from your local containers to services and RDS instances in your environment. [See detailed section](#proxy-with-copilot-run-local)
- **`copilot run local --watch`**: Automatically rebuilds your containers when you make changes to your code. [See detailed section](#watch-flag-for-copilot-run-local)
- **Importing ALBs**: You can front your Load-Balanced Web Services with existing ALBs. [See detailed section](#imported-albs)

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision Infrastructure as Code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Proxy with `copilot run local`

The new `--proxy` flag on `copilot run local` enhances your local development experience by making it possible for your local containers to talk to services deployed in your environment.

For example, imagine you have an environment with two services, `users` and `orders`, that both have [Service Connect](../docs/manifest/lb-web-service.md#network-connect) enabled.
Additionally, `orders` has an [RDS addon](../docs/developing/addons/workload.md) deployed that it uses for storing data.
You can now run `copilot run local --proxy --name orders` and your local `orders` containers will be able to reach:

- The `users` service, using its service connect URL (defaults to `http://users:<port>`)
- `orders`'s RDS database, using any DB instance url (like `app-env-orders-random-characters.us-west-2.rds.amazonaws.com:5432`) or DB cluster URL.

## Watch flag for `copilot run local`

The `--watch` flag watches your workspace and rebuilds your containers when you make changes to your code so you can develop continuously. This is extremely useful when used with `--proxy`, as it allows you to save the overhead time required to set up proxy each time you would rebuild your application!

## Imported ALBs

Copilot now supports the new field `http.alb` in the [Load-Balanced Web Service manifest](../docs/manifest/lb-web-service.en.md). Rather than letting Copilot create a new Application Load Balancer in your environment to be shared among all load-balanced services, you may designate an existing public-facing ALB for a specific Load-Balanced Web Service (LBWS). Specify the ARN or name of an ALB from your VPC in your LBWS manifest:

```yaml
http:
  alb: [name or ARN]
```
For imported ALBs, Copilot does not manage DNS-related resources like certificates.  