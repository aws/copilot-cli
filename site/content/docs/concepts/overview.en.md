# Concepts
Copilot makes it super easy to set up and deploy your containers on AWS - but getting started is only the first step of the journey. What happens when you want to have one copy of your service running only for testing and another copy serving production traffic? What happens when you want to add another service? How do you manage deploying to all of these services? Copilot wants to help you with all of these things so let's jump into some of Copilot's core concepts to understand how they can help.

## [Applications](./applications.en.md)

An Application is a collection of services and environments. When you get started with Copilot, the first thing you'll be asked to do is choose an application name. This can be a high level description of the product you're trying to build. An example might be an application named _"chat"_ which has two services _"frontend"_ and _"api"_. These two services could then be deployed to a _"test"_ and _"production"_ environment.

## [Environments](./environments.en.md)

Rumor has it, there are people out there that can write perfect code on the first go without any bugs. While we tip our hats to those folks, we believe it's important to be able to test new code on a non-customer facing version of your service before promoting to production. In Copilot we do this by using _environments_. Each environment can have its own version of a service running allowing you to create a "test" and "production" environment. You can deploy your service to the test environment, make sure everything looks good, then deploy to your production environment. Since each environment is independent, if you deploy a bug to your test environment, customers using a service deployed to your production environment will be fine.

Until now we've been talking about just one service, but what happens when you want to add another service? Perhaps you want to add a backend service to complement your frontend service. Each environment contains a set of resources shared between all the deployed services - these resources include the network (VPC, Subnets, Security Groups, etc...), the ECS Cluster, and the load balancer. If you deploy both your frontend and backend service to your test environment, both services will share the same network and cluster.

## [Services](./services.en.md)

A service is your code and all of the supporting infrastructure needed to get it up and running on AWS. When you first get started setting up a service, Copilot will ask you what _type_ of service you want to create. The _type_ of service determines the infrastructure that'll be created to support your code. If you want your code to serve traffic from the internet, for example, Copilot can set up an Application Load Balancer and an Amazon ECS Service running on AWS Fargate.

Once you've told Copilot what type of service you're building, Copilot will take care of building your code's Dockerfile and storing the images securely in an Amazon ECR repository. Copilot will also create a simple file called the _manifest_ which contains all the knobs and toggles for your service. This includes things like how much memory and CPU should be allocated to each copy of your service, how many copies of your service you want running, and more.

## [Jobs](./jobs.en.md)

Jobs are _ephemeral_ Amazon ECS tasks that are triggered by an event. Once their work is done, the task terminates. Just like services, Copilot will ask you all the necessary information 
to quickly get going with a scheduled task on AWS. The manifest file can always be used to adjust the configuration and provide more advanced settings. 

## [Pipelines](./pipelines.en.md)

Now that you've got an application with a few services deployed to a couple of environments, staying on top of those deployments can become tricky. Copilot can help by setting up a release pipeline that deploys your service whenever you push to your git repository. (At this time, Copilot supports GitHub, Bitbucket, and CodeCommit repositories.) When a push is detected, your pipeline will build your service, push the image to ECR, and deploy to your environments.

A common pattern is to set up a pipeline for a particular service that deploys to a test environment, runs automated testing, then deploys to the production environment.

