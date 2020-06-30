---
title: "Getting started"
linkTitle: "Getting started"
weight: 4
---

Copilot is a tool to help you create, develop and manage your containerized apps on Amazon ECS. To get started, all you need is an AWS account, a Dockerfile, the AWS CLI and Copilot installed.

#### Deploying in one command

Make sure you have the AWS command line tool installed and have already run `aws configure` before you start.

To deploy your app in one command, run the following in your terminal:

```bash
$ git clone git@github.com:aws-samples/amazon-ecs-cli-sample-app.git demo-app
$ cd demo-app                                                                  
$ copilot init                                                                     \
  --app demo                                                                     \
  --service api                                                                  \
  --service-type 'Load Balanced Web Service'                                     \
  --dockerfile './Dockerfile'                                                    \
  --port 80                                                                      \
  --deploy
```


### Deploying, step by step

To get started with a sample app, you can follow the instructions below:

#### Step 1: Configure your credentials
Copilot uses [profiles](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) to connect with your AWS account, so before you get started make sure to run _aws configure_. This will set up a default profile, which Copilot will use to manage your application and services.

```bash
$ aws configure
```

#### Step 2: Get some code
With Copilot, you can easily deploy your containerized service. To get you up and running, you can clone our simple demo service. It's just a simple Flask service and a Dockerfile that describes how to deploy it. Feel free to poke around or update the code.

```bash
$ git clone git@github.com:aws-samples/amazon-ecs-cli-sample-app.git demo-app && cd demo-app
```

#### Step 3: Set up your Application
Now that you've got some awesome code, what better to do next than deploy it? We know that most interesting containerized apps consist of more than just one single service, so Copilot organizes related services into an _application_. Running the _init_ command will walk you through setting up an app.

```bash
$ copilot init
```
You'll be prompted to enter the name of an app - _demo_ is a good choice. Your application is a collection of all your related services and the environments you run those services in. We'll talk a bit more about apps, services and environments later in the _concepts_ section.

#### Step 4: Set up your service
Rather than require you to manually configure exactly which resources you need, what type of load balancer or which VPC configuration to use, you can tell us the _kind_ of service you're building. In our case, we're building a simple Flask API service so let's select a `Load Balanced Web Service`.

After that, you'll be prompted to enter a service name and select a Dockerfile. Let's call your service _api_.

Once you press _enter_, Copilot will spin up some resources to create ECR repositories, S3 buckets and KMS keys. Copilot uses these resources to securely store your container images and configuration. Once those resources are spun up, you'll be asked if you want to deploy to a test environment. Select _yes_ and Copilot will spin up your network, ECS cluster and services, an Application Load Balancer and will start a deployment to your service stack.

<img src="https://user-images.githubusercontent.com/828419/69770895-91813f80-113f-11ea-8be9-60df6c2bf3fc.gif" class="img-fluid">

> A sped up view of setting up a hello-world project and a front-end app


#### Step 5: Cleaning up

To delete and clean up all the resources, run:

```bash
$ copilot app delete --env-profiles test=default
```

This will delete all the services and environments in your app.