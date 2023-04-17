An application is a group of related services, environments, and pipelines. Whether you have one service that does everything or a constellation of micro-services, Copilot organizes the service(s), and the environments into which they can be deployed, into an "application."

Let's walk through an example. We want to build a voting app which needs to collect votes and aggregate the results.

To set up our vote app with two services, we can run `copilot init` twice. The first time we run `copilot init`, we'll be asked what we should call the application this service will belong to. Since we're trying to build a voting system, we can call our application "vote" and our first service "collector". The next time we run `init`, we'll be asked if we want to add our new service to the existing “vote” app, and we’ll name the new service "aggregator".

Your application configuration (which services and environments belong to it) is stored in your AWS account, so any other users in your account will be able to develop on the “vote" app as well. This means that you can have a teammate work on one service while you develop the other.

![](https://user-images.githubusercontent.com/879348/85869625-cd858d00-b780-11ea-817c-638814049d2d.png)

## Creating an App

!!! Attention
    If you have an existing `copilot/` directory that you created for other purposes, you may find Copilot creating files in that directory. If this happens, you can make an empty directory also named `copilot/` closer to your working directory. Copilot will use this empty directory instead.

To set up an application, you can just run `copilot init`. You'll be asked if you want to set up an app or choose an existing app.

```bash
copilot init
```

Once you've created an application, Copilot stores that application in SSM Parameter store in your AWS account. The account used to set up your application is known as the "application account". This is where your app's configuration lives, and anyone who has access to this account can use this app.

All resources created within this application will be tagged with the `copilot-app` [aws resource tag](https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html). This helps you know which app resources in your account belong to.

The name of your application has to be unique within your account (even across region).

### Additional App Configurations
You can also provide more granular configuration for your application by running `copilot app init`. This includes options to:

* Tag all application, service and environment resources with an additional set of [aws resource tags](https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html)
* Use a custom domain name for Load Balanced services
* Pass in an existing IAM policy with which to set a permissions boundary for all roles created within the application.

```bash
$ copilot app init                                \
  --domain my-awesome-app.aws                     \
  --resource-tags department=MyDept,team=MyTeam   \
  --permissions-boundary my-pb-policy
```

## App Infrastructure

While the bulk of the infrastructure Copilot provisions is specific to an environment and service, there are some application-wide resources as well.

![](https://user-images.githubusercontent.com/879348/85869637-d0807d80-b780-11ea-8359-6d75933c562a.png)

### ECR Repositories
ECR Repositories are regional resources which store your service images. Each service has its own ECR Repository per region in your app.

In the above diagram, the app has several environments spread across three regions. Each of those regions has its own ECR repository for every service in your app. In this case, there are three services.

Every time you add a service, we create an ECR Repository in every region. We do this to maintain region isolation (if one region goes down, environments in other region won't be affected) and to reduce cross-region data transfer costs.

These ECR Repositories all live within your app's account (not the environment accounts) - and have policies which allow your environment accounts to pull from them.

### Release Infrastructure
For every region represented in your app, we create a KMS Key and an S3 bucket. These resources are used by CodePipeline to enable cross-region and cross-account deployments. All pipelines in your app share these same resources.

Similar to the ECR Repositories, the S3 bucket and KMS keys have policies which allow for all of your environments, even in other accounts, to read encrypted deployment artifacts. This makes your cross-account, cross-region CodePipelines possible.

## Digging into your App

Now that we've set up an app, we can check on it using Copilot. Below are a few common ways to check in on your app.

### What applications are in my account?

To see all the apps in your current account and region you can run `copilot app ls`.

```bash
$ copilot app ls
vote
ecs-kudos
```

### What's in my application?

Running `copilot app show` will show you a summary of your application, including all the services and environments in your app.

```console
$ copilot app show
About

  Name              vote
  Version           v1.1.0 
  URI               vote-app.aws

Environments

  Name              AccountID           Region
  ----              ---------           ------
  test              000000000000        us-east-1

Workloads

  Name              Type                        Environments
  ----              ----                        ------------
  collector         Load Balanced Web Service   prod
  aggregator        Backend Service             test, prod

Pipelines

  Name
  ----
```
