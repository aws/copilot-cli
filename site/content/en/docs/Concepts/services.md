---
title: "Services"
date: 2017-01-05
weight: 4
---
One of the awesome things about containers is that once you've written your code, running it locally is as easy as typing  _docker run_. 
Copilot makes running those same containers on AWS as easy as running _copilot init_. 
Copilot will build your image, push it to Amazon ECR and set up all the infrastructure to run your service in a scalable and secure way.
  
### Creating a Service

Creating a service to run your containers on AWS can be done in a few ways. The easiest way is by running the _init_ command from the same directory as your Dockerfile.

```bash
$ copilot init
```

You'll be asked which application do you want this service to be a part of (or to create an application if there isn't one). Copilot will then ask about the __type__ of service you're trying to build.

After selecting a service type, Copilot will detect any health checks or exposed ports from your Dockerfile and ask if you'd like to deploy.

### Choosing a Service Type

We mentioned before that Copilot will set up all the infrastructure your service needs to run. But how does it know what kind of infrastructure to use?

When you're setting up a service, Copilot will ask you about what kind of service you want to build. Do you want your service to serve internet traffic? You can select a __Load Balanced Web Service__ and Copilot will provision an application load balancer, security groups, an ECS Service and run your service on Fargate.

If you want a service that can't be accessed externally, but only from other services within your application, you can create a __Backend Service__. Copilot will provision an ECS Service running on AWS Fargate, but won't set up any internet-facing endpoints.

Currently these are the service types supported:
* Load Balanced Web Service
* Backend Service

### Config and the Manifest

After you've run _copilot init_ you might have noticed that Copilot created a file called `manifest.yml` in the copilot directory. This manifest file contains common configuration options for your service. While the exact set of options depends on the type of service you're running, common ones include the resources allocated to your service (like memory and CPU), health checks, and environment variables.

Let's take a look at the manifest for a Load Balanced Web Service called _front-end_.

```yaml
name: front-end
type: Load Balanced Web Service

image:
  # Path to your service's Dockerfile.
  build: ./Dockerfile
  # Port exposed through your container to route traffic to it.
  port: 8080

http:
  # Requests to this path will be forwarded to your service.
  # To match all requests you can use the "/" path.
  path: '/'
  # You can specify a custom health check path. The default is "/"
  # healthcheck: '/'

# Number of CPU units for the task.
cpu: 256
# Amount of memory in MiB used by the task.
memory: 512
# Number of tasks that should be running in your service.
count: 1

# Optional fields for more advanced use-cases.
#
variables:                    # Pass environment variables as key value pairs.
  LOG_LEVEL: info

#secrets:                         # Pass secrets from AWS Systems Manager (SSM) Parameter Store.
#  GITHUB_TOKEN: GH_SECRET_TOKEN  # The key is the name of the environment variable,
                                  # the value is the name of the SSM parameter.

# You can override any of the values defined above by environment.
environments:
  prod:
    count: 2               # Number of tasks to run for the "prod" environment.
```

#### Image

The image section contains just a few parameters, the location of the Dockerfile and the port exposed. For a Load Balanced Web Service, the port is where traffic is forwarded to. Copilot will auto-populate this field if there's an `EXPOSE` command in your Dockerfile.

#### HTTP

The HTTP section is unique to the Load Balanced Web Service type. When a request comes to the load balancer, traffic will be forwarded to this service if the path matches '/' - meaning any traffic will be forwarded to this service. You could update this so that only traffic to the _front-end_ path would be routed to this service by updating the path to be `path: 'front-end'`.

There's also an optional health check path. This path is invoked every couple of seconds so that the load balancer can ensure your service is healthy. By default the health check path is `/` - but this can be changed to anything.

#### Scaling

The next sectionin includes the resources allocated to your service. Load Balanced Web Services are run on AWS Fargate, meaning all you have to do is say how much CPU and memory your service needs. This section also includes how many copies of your service you want up and running. By default, only one copy of your service is spun up, but that number can be increased to handle more load.

#### Variables

The variable section includes environment variables that will be passed to your service. There are a number of environment variables that are passed in by default.

#### Secrets

The secrets section let's you pass in secret values to your service as environment variables, securely. In this example, the environment variable `GITHUB_TOKEN` will be set in your service and the value will be extracted from an AWS SSM Parameter named `GH_SECRET_TOKEN`.

#### Environments

The environment section lets you overwrite any value in your manifest based on the environment you're in. In the example manifest above, we're overriding the _count_ parameter so that we can run 2 copies of our service in our _prod_ environment.

### Deploying a Service

Once you've set up your service, you can deploy it (and any changes to your manifest) by running the deploy command:

```bash
$ copilot deploy
```

Running this command will:
1. Build your image locally
2. Push to your service's ECR repository
3. Convert your manifest file to CloudFormation
4. Package any additional infrastructure into CloudFormation
5. Deploy your updated service and resources to CloudFormation

If you have multiple environments, you'll be prompted to select which environment you want to deploy to.

### Digging into your Service

Now that we've got a service up and running, we can check on it using Copilot. Below are a few common ways to check in on your deployed service.

#### What's in your service?

Running `copilot svc show` will show you a summary of your service. Here's an example of the output you might see for a load balanced web application. This output includes the configuration of your service for each environment, all the endpoints for your service, and the environment variables passed into your service. You can also provide an optional `--resources` flag to see all AWS resources associated with your service.

```bash
$ copilot svc show
About

  Application       my-app
  Name              front-end
  Type              Load Balanced Web Service

Configurations

  Environment       Tasks               CPU (vCPU)          Memory (MiB)        Port
  test              1                   0.25                512                 80

Routes

  Environment       URL
  test              http://my-ap-Publi-1RV8QEBNTEQCW-1762184596.ca-central-1.elb.amazonaws.com

Service Discovery

  Environment       Namespace
  test              front-end.my-app.local:8080

Variables

  Name                                Environment         Value
  COPILOT_APPLICATION_NAME            test                my-app
  COPILOT_ENVIRONMENT_NAME            test                test
  COPILOT_LB_DNS                      test                my-ap-Publi-1RV8QEBNTEQCW-1762184596.ca-central-1.elb.amazonaws.com
  COPILOT_SERVICE_DISCOVERY_ENDPOINT  test                my-app.local
  COPILOT_SERVICE_NAME                test                front-end
```

#### What's your service status?

Often it's handy to be able to check on the status of your service. Are all the instances of my service healthy? Are there any alarms firing? To do that, you can run `copilot svc status` to get a summary of your service's status.


```bash
$ copilot svc status
Service Status

  ACTIVE 1 / 1 running tasks (0 pending)

Last Deployment

  Updated At        12 minutes ago
  Task Definition   arn:aws:ecs:ca-central-1:693652174720:task-definition/my-app-test-front-end:1

Task Status

  ID                Image Digest        Last Status         Health Status       Started At          Stopped At
  37236ed3          da3cfcdd            RUNNING             HEALTHY             12 minutes ago      -

Alarms

  Name              Health              Last Updated        Reason
  CPU-Utilization   OK                  5 minutes ago       -
```

#### Where are my service logs?

Checking the your service logs is easy as well. Running `copilot svc logs` will show the most recent logs of your service. You can follow your logs live with the `--follow` flag.

```bash
$ copilot svc logs
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
```
