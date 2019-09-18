# Amazon ECS CLI 2.0

The ECS CLI 2.0 is a completely different tool than 1.0. The first version took an operational lens to managing projects. 
You interacted directly with ECS primitives. In 2.0, we shift our focus from using Compose to model tasks and services 
to helping you model your entire ECS applications more holistically. 
ECS CLI 2.0 is our opinionated view of **application-first development**. 

## 🌟 Tenets 
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
While the ECS CLI can be used to manually deploy changes to an application, we always help customers to move to CI/CD by helping them set up and manage pipelines. ECS CLI fits into a variety of CI/CD tooling like Jenkins to continuously update microservices to their desired state stored in a source code repository.

## 📚 Documentation
For details on how to use ECS CLI, checkout out our documentation.

## 📝 License
This library is licensed under the Apache 2.0 License. 
