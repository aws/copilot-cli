# AWS Copilot v1.18: Certificates import, ordering deployments in a pipeline and more

The AWS Copilot core team is announcing the Copilot v1.18 release.
Special thanks to [@corey-cole](https://github.com/corey-cole) who contributed for this release. Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has almost 280 people online and over 2.2k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.18 brings several new features and improvements:

* **Certificates import:** You can now run `copilot env init --import-cert-arns` to import validated ACM certificates to your environment for the load balancer listener. [See detailed section](./#certificates-import).
* **Ordering deployments in a pipeline:** When use pipeline to deploy, now you are able to specify dependencies between their workload deployments, making it possible to deploy your services in order. [See detailed section](./#ordering-deployments-in-a-pipeline).
* **More options for deployment configuration** You can now specify deployment strategy to be "recreate" so new tasks won't get spun up until old tasks stop. [See detailed section](./#more-options-for-deployment-configuration).
* **Load balanced web service, Worker Service, and Backend Service observability** To support collecting and shipping traces to AWS X-Ray from ECS Tasks, we are introducing a manifest option that will add the [AWS Distro for OpenTelemetry Collector](https://github.com/aws-observability/aws-otel-collector) as a sidecar container. [See detailed section](./#load-balanced-web-service-worker-service-and-backend-service-observability).

## What’s AWS Copilot?

The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.  
From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code in a single operation.
Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro services creation and operation,
enabling you to focus on developing your application, instead of writing deployment scripts.

See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Certificates Import
_Contributed by [Penghao He](https://github.com/iamhopaul123/)_

## Ordering Deployments in a Pipeline
_Contributed by [Efe Karakus](https://github.com/efekarakus/)_

## More Options for Deployment Configuration
_Contributed by [Parag Bhingre](https://github.com/paragbhingre/)_

## Load Balanced Web Service, Worker Service, and Backend Service Observability
_Contributed by [Danny Randall](https://github.com/dannyrandall/)_

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

* Download [the latest CLI version](../docs/getting-started/install.en.md)
* Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
* Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.16.0)
