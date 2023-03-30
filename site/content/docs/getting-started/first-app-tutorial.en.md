AWS Copilot makes it easy to deploy your containers to AWS in just a few steps. In this tutorial we’re going to do just that - we’re going to deploy a sample front end service that you can visit in your browser. While we’ll be using a sample static website in this example, you can use AWS Copilot to build and deploy any container app with a Dockerfile. After we get your service all set up, we’ll show you how to delete the resources Copilot created to avoid charges.

Sound fun? Let’s do it!

## Step 1: Download & Configure AWS Copilot

You’ll need a few things to use AWS Copilot - the AWS Copilot binary, AWS CLI, Docker Desktop (needs to be running) and AWS credentials.

Follow our instructions here on how to set up and configure all these tools.

Make sure that you have a `default` profile! Run `aws configure` to set one up!

## Step 2: Download some code to deploy

In this example, we’ll be using a sample app that’s just a simple static website - but if you already have something you’d like to deploy, just open your terminal and `cd` into your Dockerfile’s directory.

Otherwise you can just clone our sample repository. In your terminal, copy and paste this code. This will clone our sample app and change directories to it.


```bash
$ git clone https://github.com/aws-samples/aws-copilot-sample-service example
$ cd example
```

## Step 3: Set up our app

Now this is where the fun starts! We have our service code and our Dockerfile and we want to get it deployed to AWS. Let’s have AWS Copilot help us do just that!

!!! Attention
    If you have an existing `copilot/` directory that you created for other purposes, you may find Copilot creating files in that directory. If this happens, you can make an empty directory also named `copilot/` closer to your working directory. Copilot will use this empty directory instead.

From within your code directory run:

```bash
$ copilot init
```

<img width="826" alt="gettingstarted" src="https://user-images.githubusercontent.com/879348/86040246-8d304400-b9f8-11ea-9590-2878c3a1d3de.png">

## Step 4: Answer a few questions

The next thing we’re going to do is answer a few questions from Copilot. Copilot will use these questions to help us choose the best AWS infrastructure for your service. There’s only a few so let’s go through them:


1. _“What would you like to name your application”_ - an application is a collection of services. In this example we’ll only have one service in our app, but if you wanted to have a multi-service app, Copilot makes that easy. Let’s call this app **example-app**.
2. _“Which service type best represents your service's architecture?”_ - Copilot is asking us what we want our service to do - do we want it to service traffic? Do we want it to be a private backend service? For us, we want our app to be accessible from the web, so let's hit 'Enter' to select **Load Balanced Web Service**.
3. _“What do you want to name this Load Balanced Web Service?”_ - now what should we call our service in our app? Be as creative as you want - but I recommend naming this service **front-end**.
4. _“Which Dockerfile would you like to use for front-end?”_ - go ahead and choose the default Dockerfile here. This is the service that Copilot will build and deploy for you.

Once you choose your Dockerfile, Copilot will start setting up the AWS infrastructure to manage your service.
<img width="826" alt="init" src="https://user-images.githubusercontent.com/879348/86040314-ab963f80-b9f8-11ea-8de6-c8caea8f6abf.png">
## Step 5: Deploy your service

Once Copilot finishes setting up the infrastructure to manage your app, you’ll be asked if you want to deploy your service to a test environment type **yes.**

Now we can wait a few minutes ⏳ while Copilot sets up all the resources needed to run your service. After all the infrastructure for your service is set up, Copilot will build your image and push it to Amazon ECR, and start deploying to Amazon ECS on AWS Fargate.

After your deployment completes, your service will be up and running and Copilot will print a link to the URL 🎉!

<img width="834" alt="deploy" src="https://user-images.githubusercontent.com/879348/86040356-be107900-b9f8-11ea-82cd-3bf2a5eb5c9d.png">

## Step 6: Clean up

Now that you've deployed your service, let's go ahead and run `copilot app delete` - this will delete all the resources Copilot set up for your application, including your ECS Service and the ECR Repository. To delete everything run:

```bash
$ copilot app delete
```

<img width="738" alt="delete" src="https://user-images.githubusercontent.com/879348/86040380-c9fc3b00-b9f8-11ea-87c2-6d42518d39dd.png">

## Congratulations!

Congratulations! You have learned how to use AWS Copilot to set up your container application, deploy it to Amazon ECS on AWS Fargate, and delete it. AWS Copilot is a command line tool that helps you develop, release and operate your container apps on AWS.

We hope you had fun deploying your app. Ready to dive deeper into AWS Copilot and learn how to build and manage production ready container apps on AWS? Check out the _Developing_ section in the sidebar.
