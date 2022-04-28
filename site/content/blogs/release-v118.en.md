# AWS Copilot v1.18: Certificate import, ordering deployments in a pipeline, and more

The AWS Copilot core team is announcing the Copilot v1.18 release.
Special thanks to [@corey-cole](https://github.com/corey-cole) who contributed to this release. Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has almost 280 people online and over 2.2k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.18 brings several new features and improvements:

* **Certificate import:** You can now run `copilot env init --import-cert-arns` to import validated ACM certificates to your environment's load balancer listener. [See detailed section](./#certificate-import).
* **Ordering deployments in a pipeline:** When using pipelines to deploy, you are now able to specify dependencies between workloads so that they are deployed in order. [See detailed section](./#ordering-deployments-in-a-pipeline).
* **"recreate" strategy for faster deployments** You can now specify "recreate" deployment strategy so that ECS will stop old tasks in your service before starting new ones. [See detailed section](./#recreate-strategy-for-faster-deployments).
* **Tracing for Load Balanced Web Service, Worker Service, and Backend Service** To collect and ship traces to AWS X-Ray from ECS tasks, we are introducing "observability.tracing" configuration in the manifest to add a [AWS Distro for OpenTelemetry Collector](https://github.com/aws-observability/aws-otel-collector) sidecar container. [See detailed section](./#tracing-for-load-balanced-web-service-worker-service-and-backend-service).

## What’s AWS Copilot?

The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.  
From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code in a single operation.
Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro services creation and operation,
enabling you to focus on developing your application, instead of writing deployment scripts.

See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Certificate import
_Contributed by [Penghao He](https://github.com/iamhopaul123/)_

## Ordering Deployments in a Pipeline
_Contributed by [Efe Karakus](https://github.com/efekarakus/)_

## "recreate" Strategy for Faster Deployments
_Contributed by [Parag Bhingre](https://github.com/paragbhingre/)_

## Tracing for Load Balanced Web Service, Worker Service, and Backend Service
_Contributed by [Danny Randall](https://github.com/dannyrandall/)_

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

* Download [the latest CLI version](../docs/getting-started/install.en.md)
* Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
* Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.18.0)
