---
title: 'AWS Copilot v1.24: ECS Service Connect!'
twitter_title: 'AWS Copilot v1.24'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.24: ECS Service Connect!

Posted On: Nov 28, 2022

The AWS Copilot core team is announcing the Copilot v1.24 release.   
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has over 350 people online and over 2.5k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.24 brings several new features and improvements:

- **ECS Service Connect support**: [See detailed section](#ecs-service-connect-support).
- **Add `--no-rollback` flag to `env deploy`**: Copilot `env deploy` now has a new flag `--no-rollback`; you can specify the flag to disable automatic env deployment rollback to help with debugging.
- **Config autoscaling for Request-Driven Web Service**: It is now possible to specify autoscaling configuration for your RDWS. For example, this can be configured in your service manifest:
```yaml
count: high-availability/3
```
- **Add log retention to VPC flow logs**: There is now a default value of 14 days.
```yaml
network:
  vpc:
    flow_logs: on
```
 Alternatively, you can customize the number of days for retention:
```yaml
network:
  vpc:
    flow_logs:
      retention: 30
```


???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## ECS Service Connect Support
[Copilot supports](../docs/developing/svc-to-svc-communication.en.md#service-connect) the newly launched [ECS Service Connect](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html)! Your private service-to-service communication will be more resilient and load-balanced with Service Connect than with Service Discovery. Let's walk through how Copilot supports ECS Service Connect.

### (Optional) Deploy an example service
If you don't have any existing services deployed, please follow [our tutorial](../docs/getting-started/first-app-tutorial.en.md) to deploy a simple front-end service that is accessible in your browser.

### Set up Service Connect
In addition to Service Discovery, you can set up Service Connect with the following configuration in your service manifest.

```yaml
network:
  connect: true
```

!!! attention
    In order to use Service Connect, both server and client services need to have Service Connect enabled.

### Check out the generated endpoint
After successfully deploying with the updated manifest, Service Connect should be enabled for your service. You can run `copilot svc show` to get the endpoint URL for your service.

```
$ copilot svc show --name front-end

...
Internal Service Endpoints

  Endpoint                      Environment  Type
  --------                      -----------  ----
  front-end:80                  test         Service Connect
  front-end.test.demo.local:80  test         Service Discovery
...
```
As shown above, `front-end:80` is your Service Connect endpoint that your other client services can call. (They must have Service Connect enabled as well.)

### (Optional) Verify that it works
To verify the IP address for your Service Connect endpoint URL has indeed been added to your service network, you can simply use `copilot svc exec` to execute into your container and check out the hosts file.

```
$ copilot svc exec --name front-end
Execute `/bin/sh` in container frontend in task a2d57c4b40014a159d3b2e3ec7b73004.

Starting session with SessionId: ecs-execute-command-088d464a5721fuej3f
# cat /etc/hosts
127.0.0.1 localhost
10.0.1.253 ip-10-0-1-253.us-west-2.compute.internal
127.255.0.1 front-end
2600:f0f0:0:0:0:0:0:1 front-end
# exit


Exiting session with sessionId: ecs-execute-command-088d464a5721fuej3f.
```

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.24.0)
