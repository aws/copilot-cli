When you first run `copilot init`, you're asked if you want to create a _test_ environment. This test environment contains all the AWS resources to provision a secure network (VPC, subnets, security groups, and more), as well as other resources that are meant to be shared between multiple services like an Application Load Balancer or an ECS Cluster. When you deploy your service into your _test_ environment, your service will use the _test_ environment's network and resources. Your application can have multiple environments, and each will have its own networking and shared resources infrastructure.

While Copilot creates a test environment for you when you get started, it's common to create a new, separate environment for production. This production environment will be completely independent from the test environment, with its own networking stack and its own copy of services. By having both a test environment and a production environment, you can deploy changes to your test environment, validate them, then promote them to the production environment.

In the diagram below we have an application called _MyApp_ with two services, _API_ and _Backend_. Those two services are deployed to the two environments, _test_ and _prod_. You can see that in the _test_ environment, both services are running only one container while the _prod_ services have more containers running. Services can use different configurations depending on the environment they're deployed in. For more, check out the [using environment variables](../developing/environment-variables.md) guide.

![](https://user-images.githubusercontent.com/879348/85873795-7da9c480-b786-11ea-9990-9604a3cc5f01.png)

## Creating an Environment

To create a new environment in your app, you can run `copilot env init` from within your workspace. Copilot will ask you what you want to name this environment and what profile you'd like to use to bootstrap the environment. These profiles are AWS [named profiles](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) which are associated with a particular account and region. When you select one of these profiles, your environment will be created in whichever account and region that profile is associated with.


```bash
$ copilot env init
```

After you run `copilot env init` you can watch as Copilot sets up all the environment resources, which can take a few minutes. Once all those resources are created, the environment will be linked back to the application account. This allows actors in the application account to manage the environment, even without access to the environment account. This linking process also creates and configures new regional ECR repositories, if necessary.


### Deploying a Service

When you first create a new environment, no services are deployed to it. To deploy a service run `copilot deploy` from that service's directory, and you'll be prompted to select which environment to deploy to.

## Environment Infrastructure

![](https://user-images.githubusercontent.com/879348/85873802-800c1e80-b786-11ea-8b2c-779b01abbaf4.png)


### VPC and Networking

Each environment gets its own multi-AZ VPC. Your VPC is the network boundary of your environment, allowing the traffic you expect in and out, and blocking the rest. The VPCs Copilot creates are spread across two availability zones to help balance availability and cost - with each AZ getting a public and private subnet.

Your services are launched in the public subnets but can be reached only through your load balancer.

###  Load Balancers and DNS

If you set up any service using one of the Load Balanced Service types, Copilot will set up an Application Load Balancer. All Load Balanced Web Services within an environment will share a load balancer by creating app-specific listeners on it. Your load balancer is allowed to communicate with services in your VPC.

Optionally, when you set up an application, you can provide a domain name that you own and is registered in Route 53. If you provide a domain name, each time you spin up an environment, Copilot will create a subdomain environment-name.app-name.your-domain.com, provision an ACM cert, and bind it to your Application Load Balancer so it can use HTTPS.

## Customize your Environment
Optionally, you can customize your environment interactively by using flags to import your existing resources, or configure the default environment resources. Currently, only VPC resources are customizable. However, if you want to customize more types of resources, feel free to bring your use cases and cut an issue!

## Digging into your Environment

Now that we've spun up an environment, we can check on it using Copilot. Below are a few common ways to check in on your environment.

### What environments are part of my app?

To see all the environments in your application you can run `copilot env ls`.

```bash
$ copilot env ls
test
production
```

### What's in your environment?

Running `copilot env show` will show you a summary of your environment. Here's an example of the output you might see for our test environment. This output includes the account and region the environment is in, the services deployed to that environment, and the tag that all resources created in this environment will have. You can also provide an optional `--resources` flag to see all AWS resources associated with this environment.

```bash
$ copilot env show --name test
About

  Name              test
  Production        false
  Region            us-west-2
  Account ID        693652174720

Services

  Name              Type
  ----              ----
  api               Load Balanced Web Service
  backend           Backend Service


Tags

  Key                  Value
  ---                  -----
  copilot-application  my-app
  copilot-environment  test
```
