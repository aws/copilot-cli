---
title: 'AWS Copilot v1.18: Certificate import, ordering deployments in a pipeline, and more!'
twitter_title: 'AWS Copilot v1.18'
image: 'https://user-images.githubusercontent.com/879348/166855730-541cb7d5-82e0-4255-afcc-646058cfc626.png'
image_alt: 'Controlling order of deployments in a Copilot pipeline'
image_width: '1337'
image_height: '316'
---

# AWS Copilot v1.18: Certificate import, ordering deployments in a pipeline, and more

The AWS Copilot core team is announcing the Copilot v1.18 release.
Special thanks to [@corey-cole](https://github.com/corey-cole) who contributed to this release. Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has almost 280 people online and over 2.2k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.18 brings several new features and improvements:

* **Certificate import:** You can now run `copilot env init --import-cert-arns` to import validated ACM certificates to your environment's load balancer listener. [See detailed section](./#certificate-import).
* **Ordering deployments in a pipeline:** You can now control the order in which services or jobs get deployed in a continuous delivery pipeline. [See detailed section](./#controlling-order-of-deployments-in-a-pipeline).
* **Additional pipeline improvements:** Besides deployment orders, you can now limit which services or jobs to deploy in your pipeline or deploy custom cloudformation stacks in a pipeline. [See detailed section](./#additional-pipeline-improvements).
* **"recreate" strategy for faster deployments:** You can now specify "recreate" deployment strategy so that ECS will stop old tasks in your service before starting new ones. [See detailed section](./#recreate-strategy-for-faster-deployments).
* **Tracing for Load Balanced Web, Worker, and Backend Service:** To collect and ship traces to AWS X-Ray from ECS tasks, we are introducing `observability.tracing` configuration in the manifest to add an [AWS Distro for OpenTelemetry Collector](https://github.com/aws-observability/aws-otel-collector) sidecar container. [See detailed section](./#tracing-for-load-balanced-web-service-worker-service-and-backend-service).

## What’s AWS Copilot?

The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.  
From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code in a single operation.
Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro services creation and operation,
enabling you to focus on developing your application, instead of writing deployment scripts.

See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Certificate Import
_Contributed by [Penghao He](https://github.com/iamhopaul123/)_

## Ordering Deployments in a Pipeline
_Contributed by [Efe Karakus](https://github.com/efekarakus/)_

## "recreate" Strategy for Faster Redeployments
_Contributed by [Parag Bhingre](https://github.com/paragbhingre/)_

!!!alert
    Due to the possible service downtime caused by "recreate", we do **not** recommend using it for your production services.

Before v1.18, a Copilot ECS-based service (Load Balanced Web Service, Backend Service, and Worker Service) redeployment always spun up new tasks, waited for them to be stable, and then stopped the old tasks. In order to support faster redeployments for ECS-based services in the development stage, users can specify `"recreate"` as the deployment strategy in the service manifest:

```yaml
deployment:
  rolling: recreate
```

Under the hood, Copilot sets [minimumHealthyPercent and maximumPercent](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DeploymentConfiguration.html) to `0` and `100` respectively (defaults are `100` and `200`), so that old tasks are stopped before spinning up any new tasks.

## Tracing for Load Balanced Web Service, Worker Service, and Backend Service
_Contributed by [Danny Randall](https://github.com/dannyrandall/)_

In [v1.17](./release-v117.en.md#send-your-request-driven-web-services-traces-to-aws-x-ray), Copilot launched support for sending traces from your Request-Driven Web Services to [AWS X-Ray](https://aws.amazon.com/xray/).
Now, you can easily export traces from your Load Balanced Web, Worker, and Backend services to X-Ray by modifying your service's manifest:
```yaml
observability:
  tracing: awsxray
```

For these services, Copilot will deploy an [AWS Distro for OpenTelemetry Collector](https://github.com/aws-observability/aws-otel-collector) sidecar container to collect traces from your service and export them to X-Ray.
After [instrumenting your service](../docs/developing/observability.en.md#instrumenting-your-service) to send traces, you can view the end-to-end journey of requests through your services to aid in debugging and monitoring performance of your application.

![X-Ray Service Map Example](https://user-images.githubusercontent.com/10566468/166986340-e3b7c0e2-c84d-4671-bf37-ba95bdb1d6b2.png)

Read our documentation on [Observability](../docs/developing/observability.en.md) to learn more about tracing and get started!

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

* Download [the latest CLI version](../docs/getting-started/install.en.md)
* Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
* Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.18.0)
