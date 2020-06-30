# AWS Copilot CLI Charter

## Mission
Our mission is to help customers build, release and operate applications on Amazon ECS with dev-ops best practices and production ready infrastructure patterns.

## Tenets 🌟
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
