# Amazon ECS CLI 2.0

The ECS CLI is a tool for developers to create, release and manage production ready containerized applications on ECS. 
From getting started to pushing staging and releasing to production, the ECS CLI can manage the entire lifecycle 
of your application development. Get started by bringing your own containerized app or using one of our predefined, 
production ready application templates.

Once you've built something you're excited to deploy, let the ECS CLI set up a CI/CD pipeline for you, 
with built in testing and manual gates. 
Tail your logs, monitor your key metrics and push updates all from the comfort of your terminal.

Use the ECS CLI to:
* Start from scratch with our application templates and Dockerfiles
* Bring your existing Docker apps
* Set up staging and production environments, cross region and cross account
* Set up production-ready, battle tested ECS Clusters, Services and infrastructure
* Set up CI/CD Pipelines for all of the micro-services that make up your Project
* Monitor and debug your applications

## 📚 Documentation
<!-- TODO add link -->
For details on how to use ECS CLI, checkout out our documentation.

## 🌟 Tenets (unless you know better ones)
* **Create modern applications by default.** 
Applications created with the ECS CLI use the best practices of modern applications by default: they are serverless, 
they use infrastructure-as-code, they are observable, and they are secure.
* **Users think in terms of architecture, not of infrastructure.** 
Developers creating a new microservice shouldn't have to specify VPCs, load balancer settings, or complex pipeline configuration. 
They may not know anything about other AWS services. They should be able to specify what "kind" of service it is and how 
it fits into their overall architecture; the infrastructure should be generated from that.
* **Operations is part of the workflow.** 
Modeling, provisioning, and deploying applications are only part of the application lifecycle for the developer. 
The CLI must support workflows around troubleshooting and debugging to help when things go wrong.
* **Deliver applications continuously.**
While the ECS CLI can be used to manually deploy changes to an application, we always help customers to move to CI/CD by helping them set up and manage pipelines.


## 📝 License
This library is licensed under the Apache 2.0 License. 
