Having an automated release process is one of the most important parts of software delivery, so Copilot wants to make setting up that process as easy as possible 🚀.

In this section, we'll talk about using Copilot to set up a CodePipeline that automatically builds your service code when you push to your GitHub, Bitbucket or AWS CodeCommit repository, deploys to your environments, and runs automated testing.

!!! Attention
    AWS CodePipeline is not supported for services with Windows as the OS Family.
    CodePipeline uses Linux-based AWS CodeBuild for the 'build' stage, so for now, Copilot pipelines cannot build Windows containers.

## Why?

We won't get too philosophical about releasing software, but what's the point of having a release pipeline? With `copilot deploy` you can deploy your service directly from your computer to Amazon ECS on AWS Fargate, so why add a middleman? That's a great question. For some apps, manually using `deploy` is enough, but as your release process gets more complicated (as you add more environments or add automated testing, for example) you want to offload the boring work of repeatedly orchestrating that process to a service. With two services, each having two environments (test and production, say), running integration tests after you deploy to your test environment becomes surprisingly cumbersome to do by hand.

Using an automated release tool like CodePipeline helps make your release manageable. Even if your release isn't particularly complicated, knowing that you can just `git push` to deploy your change always feels a little magical 🌈.

## Pipeline Structure

Copilot can set up a CodePipeline for you with a few commands - but before we jump into that, let's talk a little bit about the structure of the pipeline we'll be generating. Our pipeline will have the following basic structure:

1. __Source Stage__ - when you push to a configured GitHub, Bitbucket, or CodeCommit repository branch, a new pipeline execution is triggered.
2. __Build Stage__ - after your source code is pulled from your repository host, your service's container image is built and published to every environment's ECR repository and any input files, such as [addons](../developing/addons/workload.en.md) templates, lambda function zip files, and [environment variable files](../developing/environment-variables.en.md), are uploaded to S3.
3. __Deploy Stages__ - after your code is built, you can deploy to any or all of your environments, with optional manual approvals, pre- and post-deployment actions, and/or test commands.

Once you've set up a CodePipeline using Copilot, all you'll have to do is push to your GitHub, Bitbucket, or CodeCommit repository, and CodePipeline will orchestrate the deployments.

Want to learn more about CodePipeline? Check out their [getting started docs](https://docs.aws.amazon.com/codepipeline/latest/userguide/welcome-introducing.html).

## Creating a Pipeline in 3 Steps
Creating a Pipeline requires only three steps:

1. Preparing the pipeline structure.
2. Committing and pushing the files generated in the `copilot/` directory.
3. Creating the actual CodePipeline.

Follow the three steps below, from your workspace root:

```bash
$ copilot pipeline init
$ git add copilot/ && git commit -m "Adding pipeline artifacts" && git push
$ copilot pipeline deploy
```
!!! Note
    You need to have initiated at least one workload (service or job) before running `pipeline deploy`, so that your pipeline has something to deploy to your environments.

✨ And you'll have a new pipeline configured __in your application account__. Want to understand a little bit more what's going on? Read on!

## Setting Up a Pipeline, Step By Step

### Step 1: Configuring Your Pipeline

Pipeline configurations are created at a workspace level. If your workspace has a single service, then your pipeline will be triggered only for that service. However, if you have multiple services in a workspace, then the pipeline will build all the services in the workspace. To start setting up a pipeline, `cd` into your app's workspace and run:

 `copilot pipeline init`

This won't create your pipeline, but it will create some local files under `copilot/pipelines` that will be used when creating your pipeline.

* __Pipeline name__: We suggest naming your pipeline `[repository name]-[branch name]` (press 'Enter' when asked, to accept the default name). This will distinguish it from your other pipelines, should you create multiple, and works well if you follow a pipeline-per-branch workflow.

* __Release order__: You'll be prompted for environments you want to deploy to – select them based on the order you want them to be deployed in your pipeline (deployments happen one environment at a time). You may, for example, want to deploy to your `test` environment first, and then your `prod` environment.

* __Tracking repository__: After you've selected the environments you want to deploy to, you'll be prompted to select which repository you want your CodePipeline to track. This is the repository that, when pushed to, will trigger a pipeline execution. (If the repository you're interested in doesn't show up, you can pass it in using the `--url` flag.)

* __Tracking branch__: After you've selected the repository, Copilot will designate your current local branch as the branch your pipeline will follow. This can be changed in Step 2.

### Step 2: Updating the Pipeline Manifest (optional)

Just like your service has a simple manifest file, so does your pipeline. After you run `pipeline init`, two files are created: `manifest.yml` and `buildspec.yml`, both in a new `copilot/pipelines/[your pipeline name]` directory. If you poke in, you'll see that the `manifest.yml` looks something like this (for a service called "api-frontend" with two environments, "test" and "prod"):

```yaml
# The manifest for the "demo-api-frontend-main" pipeline.
# This YAML file defines your pipeline: the source repository it tracks and the order of the environments to deploy to.
# For more info: https://aws.github.io/copilot-cli/docs/manifest/pipeline/

# The name of the pipeline.
name: demo-api-frontend-main

# The version of the schema used in this template.
version: 1

# This section defines your source, changes to which trigger your pipeline.
source:
  # The name of the provider that is used to store the source artifacts.
  # (i.e. GitHub, Bitbucket, CodeCommit)
  provider: GitHub
  # Additional properties that further specify the location of the artifacts.
  properties:
    branch: main
    repository: https://github.com/kohidave/demo-api-frontend
    # Optional: specify the name of an existing CodeStar Connections connection.
    # connection_name: a-connection

# This section defines the order of the environments your pipeline will deploy to.
stages:
    - # The name of the environment.
      name: test
      test_commands:
        - make test
        - echo "woo! Tests passed"
    - # The name of the environment.
      name: prod
      # requires_approval: true
```
You can see every available configuration option for `manifest.yml` on the [pipeline manifest](../manifest/pipeline.en.md) page.

There are 3 main parts of this file: the `name` field, which is the name of your pipeline, the `source` section, which details the repository and branch to track, and the `stages` section, which lists the environments you want this pipeline to deploy to. You can update this anytime, but you must commit and push the changed files and run `copilot pipeline deploy` afterwards.

Typically, you'll update this file to add environments to deploy to, specify the order of deployments, add actions for the pipeline to run before or after deployment, or change the branch to track. You may add a manual approval step before deployment or commands to run tests (see "Adding Tests and Other Actions," below). If you are using CodeStar Connections to connect to your repository and would like to utilize an existing connection rather than let Copilot generate one for you, you may add the connection name here.

### Step 3: Updating the Buildspec (optional)

Along with `manifest.yml`, the `pipeline init` command also generated a `buildspec.yml` file in the `copilot/pipelines/[your pipeline name]` directory. This contains the instructions for building and publishing your service. If you want to run any additional commands besides `docker build`, such as unit tests or style checkers, feel free to add them to the buildspec's `build` phase.

When this buildspec runs, it pulls down the version of Copilot which was used when you ran `pipeline init`, to ensure backwards compatibility.

Alternatively, you may bring your own buildspec for CodeBuild to run. Indicate its location in [your `manifest.yml` file](../manifest/pipeline.en.md).
```yaml
build:
  buildspec:
```

### Step 4: Pushing New Files to Your Repository

Now that your `manifest.yml`, `buildspec.yml`, and `.workspace` files have been created, add them to your repository. These files in your `copilot/` directory are required for your pipeline's `build` stage to run successfully.

### Step 5: Creating Your Pipeline

Here's the fun part! Run:

`copilot pipeline deploy`

This parses your `manifest.yml`, creates a CodePipeline __in the same account and region as your application__ and kicks off a pipeline execution. Log into the AWS Console to watch your pipeline go, or run `copilot pipeline status` to check in on its execution.

![Your completed CodePipeline](https://user-images.githubusercontent.com/828419/71861318-c7083980-30aa-11ea-80bb-4bea25bf5d04.png)

!!! info
    If you have selected a GitHub or Bitbucket repository, Copilot will help you connect to your source code with [CodeStar Connections](https://docs.aws.amazon.com/dtconsole/latest/userguide/welcome-connections.html). You will need to install the AWS authentication app on your third-party account and update the connection status. Copilot and the AWS Management Console will guide you through these steps.

### Step 6: Manage Copilot Version for Your Pipeline (optional)

After creating your pipeline, you can manage the version of Copilot used by your pipeline by updating the following lines of your `buildspec.yml` to the latest version:

```yaml
...
      # Download the copilot linux binary.
      - wget -q https://ecs-cli-v2-release.s3.amazonaws.com/copilot-linux-v1.16.0
      - mv ./copilot-linux-v1.16.0 ./copilot-linux
...
```

## Adding Tests and Other Actions

Of course, one of the most important parts of a pipeline is the automated testing. To add tests, such as integration or end-to-end tests, that run after a deployment stage, include those commands in the `test_commands` section. If all the tests succeed, your change is promoted to the next stage.

Adding `test_commands` generates a CodeBuild project with the [aws/codebuild/amazonlinux2-x86_64-standard:3.0](https://docs.aws.amazon.com/codebuild/latest/userguide/build-env-ref-available.html) image - so most commands from Amazon Linux 2 (including `make`) are available for use.

Are your tests configured to run inside a Docker container? Copilot's test commands CodeBuild project supports Docker, so `docker build` commands are available as well.

In the example below, the pipeline will run the `make test` command (in your source code directory) and only promote the change to the prod stage if that command exits successfully.

```yaml
name: demo-api-frontend-main
version: 1
source:
  provider: GitHub
  properties:
    branch: main
    repository: https://github.com/kohidave/demo-api-frontend

stages:
    -
      name: test
      # A change will only deploy to the production stage if the
      # make test and echo commands exit successfully.
      test_commands:
        - make test
        - echo "woo! Tests passed"
    -
      name: prod
```
!!! info
    AWS's own Nathan Peck provides a great example of a Copilot pipeline, paying special attention to `test_commands`: https://aws.amazon.com/blogs/containers/automatically-deploying-your-container-application-with-aws-copilot/

For even more power and flexibility, add pre- and post-deployments to your pipeline manifest.
```yaml
stages:
    - name: test
      require_approval: true
      pre_deployments:
        db_migration:
          buildspec: copilot/pipelines/demo-api-frontend-main/buildspecs/buildspec.yml
      deployments:
        orders:
        warehouse:
        frontend:
          depends_on: [orders, warehouse]
      post_deployments:
        db_migration:
          buildspec: copilot/pipelines/demo-api-frontend-main/buildspecs/post_buildspec.yml
        integration:
          buildspec: copilot/pipelines/demo-api-frontend-main/buildspecs/integ-buildspec.yml
          depends_on: [db_migration]
```
Define these actions in [buildspec files]( https://docs.aws.amazon.com/codebuild/latest/userguide/build-spec-ref.html) stored locally, and add the paths, relative to your project root, to the manifest.
You may specify the run order of the actions using the `depends_on` subfield, just like you would to indicate your desired order of deployments.  

!!! info
    The CodeBuild projects generated for pre- and post-deployments are deployed in the same region as the pipeline and app. To access the VPC of the environment being deployed or deployed to, use Copilot commands in your buildspec or directly in `test_commands`.  

For more information on pre- and post-deployments, see the [v.1.30.0 blog post](../../blogs/release-v130.en.md).