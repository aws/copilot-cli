One of the awesome things about containers is that once you've written your code, running it locally is as easy as typing `docker run`. 
Copilot makes running those same containers on AWS as easy as typing `copilot init`. 
Copilot will build your image, push it to Amazon ECR and set up all the infrastructure to run your service in a scalable and secure way.
  
## Creating a Service

Creating a service to run your containers on AWS can be done in a few ways. The easiest way is by running the `init` command from the same directory as your Dockerfile.

```console
$ copilot init
```

You'll be asked which application you want this service to be a part of (or asked to create an application if there isn't one). Copilot will then ask about the __type__ of service you're trying to build.

After selecting a service type, Copilot will detect any health checks or exposed ports from your Dockerfile and ask if you'd like to deploy.

## Choosing a Service Type

We mentioned before that Copilot will set up all the infrastructure your service needs to run. But how does it know what kind of infrastructure to use?

When you're setting up a service, Copilot will ask you about what kind of service you want to build.

### Internet-facing services

If you want your service to serve internet traffic then you have two options:

* "Request-Driven Web Service" will provision an AWS App Runner Service to run your service. 
* "Load Balanced Web Service" will provision an Application Load Balancer, a Network Load Balancer or both, along with 
  security groups, an ECS service on Fargate to run your service.

#### Request-Driven Web Service
An AWS App Runner service that autoscales your instances based on incoming traffic and scales down to a baseline instance when there's no traffic. 
This option is more cost effective for HTTP services with sudden bursts in request volumes or low request volumes.

Unlike ECS, App Runner services are not connected by default to a VPC. In order to route egress traffic through a VPC, 
you can configure the [`network`](../manifest/rd-web-service.en.md#network) field in the manifest.

#### Load Balanced Web Service
An ECS Service running tasks on Fargate with an Application Load Balancer, a Network Load Balancer or both, as ingress. 
This option is suitable for HTTP or TCP services with steady request volumes that need to access resources in a VPC or 
require advanced configuration. 

Note that an Application Load Balancer is an environment-level resource, and is shared by all Load Balanced Web Services
within the environment. To learn more, go [here](environments.en.md#load-balancers-and-dns). In contrast, a Network Load Balancer 
is a service-level resource, and hence is not shared across services.

Below is a diagram for a Load Balanced Web Service that involves an Application Load Balancer only.

![lb-web-service-infra](https://user-images.githubusercontent.com/879348/86045951-39762880-ba01-11ea-9a47-fc9278600154.png)

### Backend Service
If you want a service that can't be accessed externally, but only from other services within your application, you can create a __Backend Service__. Copilot will provision an ECS Service running on AWS Fargate, but won't set up any internet-facing endpoints. To learn about creating Backend Services with internal load balancers, go [here](../developing/internal-albs.en.md).

![backend-service-infra](https://user-images.githubusercontent.com/879348/86046929-e8673400-ba02-11ea-8676-addd6042e517.png)

### Worker Service
__Worker Services__ allow you to implement asynchronous service-to-service communication with [pub/sub architectures](https://aws.amazon.com/pub-sub-messaging/). 
Your microservices in your application can `publish` events to [Amazon SNS topics](https://docs.aws.amazon.com/sns/latest/dg/welcome.html) that can then be consumed by a "Worker Service".  

A Worker Service is composed of:  

  * One or more [Amazon SQS queues](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html) to process notifications published to the topics, as well as [dead-letter queues](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html) to handle failures. 
  * An Amazon ECS service on AWS Fargate that has permission to poll the SQS queues and process the messages asynchronously.

![worker-service-infra](https://user-images.githubusercontent.com/25392995/131420719-c48efae4-bb9d-410d-ac79-6fbcc64ead3d.png)

## Config and the Manifest

After you've run `copilot init` you might have noticed that Copilot created a file called `manifest.yml` in the `copilot/[service name]/` directory. This manifest file contains common configuration options for your service. While the exact set of options depends on the type of service you're running, common ones include the resources allocated to your service (like memory and CPU), health checks, and environment variables.

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
To learn about the specification of manifest files, see the [manifest](../manifest/overview.en.md) page.

## Deploying a Service

Once you've set up your service, you can deploy it (and any changes to your manifest) by running the deploy command:

```console
$ copilot deploy
```

Running this command will:

1. Build your image locally  
2. Push to your service's ECR repository  
3. Convert your manifest file to CloudFormation  
4. Package any additional infrastructure into CloudFormation  
5. Deploy your updated service and resources to CloudFormation  

If you have multiple environments, you'll be prompted to select which environment you want to deploy to.

## Digging into your Service

Now that we've got a service up and running, we can check on it using Copilot. Below are a few common ways to check in on your deployed service.

### What's in your service?

Running `copilot svc show` will show you a summary of your service. Here's an example of the output you might see for a load balanced web application. This output includes the configuration of your service for each environment, all the endpoints for your service, and the environment variables passed into your service. You can also provide an optional `--resources` flag to see all AWS resources associated with your service.

```console
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
  test              front-end.test.my-app.local:8080

Variables

  Name                                Environment         Value
  COPILOT_APPLICATION_NAME            test                my-app
  COPILOT_ENVIRONMENT_NAME            test                test
  COPILOT_LB_DNS                      test                my-ap-Publi-1RV8QEBNTEQCW-1762184596.ca-central-1.elb.amazonaws.com
  COPILOT_SERVICE_DISCOVERY_ENDPOINT  test                test.my-app.local
  COPILOT_SERVICE_NAME                test                front-end
```

### What's your service status?

Often it's handy to be able to check on the status of your service. Are all the instances of my service healthy? Are there any alarms firing? To do that, you can run `copilot svc status` to get a summary of your service's status.


```console
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

### Where are my service logs?

Checking your service logs is easy as well. Running `copilot svc logs` will show the most recent logs of your service. You can follow your logs live with the `--follow` flag.

```console
$ copilot svc logs
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
```
