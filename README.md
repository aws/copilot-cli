[![Gitter](https://badges.gitter.im/aws/amazon-ecs-cli-v2.svg)](https://gitter.im/aws/amazon-ecs-cli-v2?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)

# Amazon ECS CLI v2

>⚠️ Before you get started please note that this feature is in preview and the intention is to ensure it meets your requirements and give us any feedback on your use case. Please do not run production workloads till we announce the general availability of this feature. Using the instructions and assets in this repository folder is governed as a preview program under the AWS Service Terms: https://aws.amazon.com/service-terms/ . 

The ECS CLI is a tool for developers to create, release and manage production ready containerized applications on ECS.
From getting started, pushing to staging and releasing to production, the ECS CLI can help manage the entire lifecycle
of your application development.

Once you've built something you're excited to deploy, let the ECS CLI set up a CI/CD pipeline for you,
with built in testing hooks and manual gates.
Tail your logs, monitor your key metrics and push updates all from the comfort of your terminal.

Use the ECS CLI to:
* Bring your existing Docker apps
* Set up staging and production environments, cross region and cross account
* Set up production-ready, battle tested ECS Clusters, Services and infrastructure
* Set up CI/CD Pipelines for all of the micro-services that make up your Project
* Monitor and debug your applications

![ecs-cli help](https://user-images.githubusercontent.com/828419/69765586-5c69f280-1129-11ea-9427-623d15975940.png)

## 🌟 Tenets (unless you know better ones)
* **Create modern applications by default.**
Applications created with the ECS CLI use the best practices of modern applications by default: they are serverless,
they use infrastructure-as-code, they are observable, and they are secure.
* **Users think in terms of architecture, not of infrastructure.**
Developers creating a new microservice shouldn't have to specify VPCs, load balancer settings, or complex pipeline configuration.
They may not know anything about other AWS services. They should be able to specify what "kind" of application it is and how
it fits into their overall architecture; the infrastructure should be generated from that.
* **Operations is part of the workflow.**
Modeling, provisioning, and deploying applications are only part of the application lifecycle for the developer.
The CLI must support workflows around troubleshooting and debugging to help when things go wrong.
* **Deliver applications continuously.**
While the ECS CLI can be used to manually deploy changes to an application, we always help customers to move to CI/CD by helping them set up and manage pipelines.

#### Security disclosures

If you think you’ve found a potential security issue, please do not post it in the Issues. Instead, please follow the instructions [here](https://aws.amazon.com/security/vulnerability-reporting/) or email AWS security directly at [aws-security@amazon.com](mailto:aws-security@amazon.com).

## 📝 License
This library is licensed under the Apache 2.0 License.
