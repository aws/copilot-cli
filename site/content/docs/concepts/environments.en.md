When you first run `copilot init`, you're asked if you want to create a _test_ environment. This test environment contains all the AWS resources to provision a secure network (VPC, subnets, security groups, and more), as well as other resources that are meant to be shared between multiple services like an Application Load Balancer or an ECS Cluster. When you deploy your service into your _test_ environment, your service will use the _test_ environment's network and resources. Your application can have multiple environments, and each will have its own networking and shared resources infrastructure.

While Copilot creates a test environment for you when you get started, it's common to create a new, separate environment for production. This production environment will be completely independent from the test environment, with its own networking stack and its own copy of services. By having both a test environment and a production environment, you can deploy changes to your test environment, validate them, then promote them to the production environment.

In the diagram below we have an application called _MyApp_ with two services, _API_ and _Backend_. Those two services are deployed to the two environments, _test_ and _prod_. You can see that in the _test_ environment, both services are running only one container while the _prod_ services have more containers running. Services can use different configurations depending on the environment they're deployed in. For more, check out the [using environment variables](../developing/environment-variables.en.md) guide.

![](https://user-images.githubusercontent.com/879348/85873795-7da9c480-b786-11ea-9990-9604a3cc5f01.png)

## Creating an Environment

To create a new environment in your app, you can run `copilot env init` from within your workspace. Copilot will ask you what you want to name this environment and what profile you'd like to use to bootstrap the environment. These profiles are AWS [named profiles](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) which are associated with a particular account and region. When you select one of these profiles, your environment will be created in whichever account and region that profile is associated with.
```console
$ copilot env init
```

After you run `copilot env init`, you can watch as Copilot sets up the two IAM roles that are essential in updating and managing the environment. If the environment was created with a different AWS account than the app, the environment will be linked back to the application account; this allows actors in the application account to manage the environment, even without access to the environment account. Copilot creates an [env manifest](../manifest/environment.en.md) at `copilot/environments/[env name]/manifest.yml`.

## Deploying an Environment

If you'd like, you may configure your environment by making changes to your [env manifest](../manifest/environment.en.md) before deploying it:
```console
$ copilot env deploy
```
In this step, Copilot creates your environment infrastructure resources, like an ECS cluster, a security group, and a private DNS namespace. After deployment, you can modify your env manifest and redeploy by simply running [`copilot env deploy`](../commands/env-deploy.en.md) again.

### Deploying a Service

When you first create a new environment, no services are deployed to it. To deploy a service run `copilot deploy` from that service's directory, and you'll be prompted to select which environment to deploy to.

## Environment Infrastructure

![](https://user-images.githubusercontent.com/879348/85873802-800c1e80-b786-11ea-8b2c-779b01abbaf4.png)


### VPC and Networking

Each environment gets its own multi-AZ VPC. Your VPC is the network boundary of your environment, allowing the traffic you expect in and out, and blocking the rest. The VPCs Copilot creates are spread across two availability zones, with each AZ getting a public and private subnet, following [AWS best practices](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-security-best-practices.html). Your services are launched by default in the public subnets while limiting ingress to only other services in your environment for security. Tasks are placed in public subnets to help manage costs by allowing egress to the internet without requiring a NAT gateway.

Workload subnet placement can be modified using the [`network.vpc.placement`](../manifest/lb-web-service.en.md#network-vpc-placement) field in the manifest.

###  Load Balancers and DNS

If you set up a Load Balanced Web Service or Backend Service with the `http` field in its manifest, Copilot will set up an Application Load Balancer to be shared among all load-balanced services within that environment. Load Balanced Web Services' ALBs are internet-facing, while Backend Services' are internal. Your load balancer is allowed to communicate with other Copilot services in your VPC.

Optionally, when you set up an application, you can provide a domain name that you own and is registered in Route 53. If you provide a domain name, each time you spin up an environment, Copilot will create a subdomain environment-name.app-name.your-domain.com, provision an ACM cert, and bind it to your Application Load Balancer so it can use HTTPS.

An internal ALB is created when a Backend Service with [`http`](../manifest/backend-service.en.md#http) configured in its manifest is deployed in an environment. For an HTTPS endpoint, use the [`--import-cert-arns`](../commands/env-init.en.md#what-are-the-flags) flag when running `copilot env init` and import a VPC with only private subnets. For more on internal ALBs, go [here](../developing/internal-albs.en.md).

If you already have an ALB in your VPC and would like to use it instead of letting Copilot create one, specify that ALB by name or ARN in your Load-Balanced Web Service's (for internet-facing ALBs) or Backend Service's (for internal ALBs) manifest before deploying it to the environment:
```yaml
http:
  path: '/'
  alb: [name or ARN]
```
Your imported ALB will be associated with only that service (and any others that similarly import it), rather than shared among all load-balanced services in the environment.  

## Customize your Environment
You can import your existing environment resources or configure the default ones by using flags with commands or by changing your [env manifest](../manifest/environment.en.md). If you want to customize more types of resources than are currently configurable, feel free to bring your use cases and cut an issue! 

For more, see our [custom environment resources](../developing/custom-environment-resources.en.md) page.

## Digging into your Environment

Now that we've spun up an environment, we can check on it using Copilot. Below are a few common ways to check in on your environment.

### What environments are part of my app?

To see all the environments in your application you can run `copilot env ls`.

```console
$ copilot env ls
test
production
```

### What's in your environment?

Running `copilot env show` will show you a summary of your environment. Here's an example of the output you might see for our test environment. This output includes the account and region the environment is in, the services deployed to that environment, and the tag that all resources created in this environment will have. You can also provide an optional `--resources` flag to see all AWS resources associated with this environment.

```console
$ copilot env show --name test
About

  Name              test
  Region            us-west-2
  Account ID        693652174720

Workloads

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
