## Amazon ECS CLI v2 - Develop, Release and Operate Container Apps on AWS

[![Join the chat at https://gitter.im/aws/amazon-ecs-cli-v2](https://badges.gitter.im/aws/amazon-ecs-cli-v2.svg)](https://gitter.im/aws/amazon-ecs-cli-v2?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

>⚠️ Before you get started please note that this feature is in preview and the intention is to ensure it meets your requirements and give us any feedback on your use case. Please do not run production workloads till we announce the general availability of this feature. Using the instructions and assets in this repository folder is governed as a preview program under the AWS Service Terms: https://aws.amazon.com/service-terms/ .


## What's the ECS CLI?

The ECS CLI is a tool for developers to create, release and manage production ready containerized applications on ECS.
From getting started, pushing to a test environment and releasing to production, the ECS CLI helps you through the entire life of your app development.

Got a Dockerfile and some code? Get it up and running on ECS in under 10 minutes, with just one command. Ready to take that app to production? Spin up new environments and a continuous delivery pipeline without having to leave your terminal. Find a bug? Tail your logs and deploy with one tool.

Use the ECS CLI to:
* Organize all your related micro-services in one project
* Set up test and production environments, across regions and accounts
* Set up production-ready, scalable ECS services and infrastructure
* Set up CI/CD Pipelines for all of the micro-services
* Monitor and debug your applications from your terminal

Read more about the ECS CLI charter and tenets [here](CHARTER.md).

![ecs-cli help](https://user-images.githubusercontent.com/828419/69765586-5c69f280-1129-11ea-9427-623d15975940.png)

## Installing

During preview, we're distributing binaries from our GitHub releases. Instructions for installing the ECS CLI v2 for your platform:

<details>
  <summary>macOS and Linux</summary>


| Platform | Command to install |
|---------|---------
| macOS | `curl -Lo /usr/local/bin/ecs-preview https://github.com/aws/amazon-ecs-cli-v2/releases/download/v0.0.7/ecs-preview-darwin-v0.0.7 && chmod +x /usr/local/bin/ecs-preview && ecs-preview --help` |
| Linux | `curl -Lo /usr/local/bin/ecs-preview https://github.com/aws/amazon-ecs-cli-v2/releases/download/v0.0.7/ecs-preview-linux-v0.0.7 && chmod +x /usr/local/bin/ecs-preview && ecs-preview --help` |

</details>


## Getting started

Make sure you have the AWS command line tool installed and have already run `aws configure` before you start.

To get a sample app up and running in one command, run the following:

```sh
git clone git@github.com:aws-samples/amazon-ecs-cli-sample-app.git demo-app
cd demo-app
ecs-preview init --project demo      \
  --app api                          \
  --app-type 'Load Balanced Web App' \
  --dockerfile './Dockerfile'        \
  --port 80                          \
  --deploy
```

This will create a VPC, Application Load Balancer, an Amazon ECS Service with the sample app running on AWS Fargate. This process will take around 8 minutes to complete - at which point you'll get a URL for your sample app running!

<details>
    <summary> watch what happens when you run <tt>init</tt></summary>

![Step By Step Setup](https://user-images.githubusercontent.com/828419/69770895-91813f80-113f-11ea-8be9-60df6c2bf3fc.gif)
</details>

### Cleaning up 🧹

Once you're finished playing around with this project, you can delete it and all the AWS resources associated it by running `ecs-preview project delete`.

### Learning more 📖

Want to learn more about what's happening? Check out our [getting started guide](https://github.com/aws/amazon-ecs-cli-v2/wiki/Getting-Started) and a detailed breakdown of all the [infrastructure](https://github.com/aws/amazon-ecs-cli-v2/wiki/Infrastructure) that gets created.

## We need your feedback 🙏

The ECS CLI v2 is in developer preview, meaning we want to know what works, what doesn't work, and what you want! Have any feedback at all? Drop us an [issue](https://github.com/aws/amazon-ecs-cli-v2/issues/new) or join us on [gitter](https://gitter.im/aws/amazon-ecs-cli-v2?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge).

We're happy to hear feedback or answer questions, so reach out, anytime!

## Security disclosures

If you think you’ve found a potential security issue, please do not post it in the Issues. Instead, please follow the instructions [here](https://aws.amazon.com/security/vulnerability-reporting/) or email AWS security directly at [aws-security@amazon.com](mailto:aws-security@amazon.com).

## License
This library is licensed under the Apache 2.0 License.
