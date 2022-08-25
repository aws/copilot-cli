---
title: 'AWS Copilot v1.20: To Environment Manifests and Beyond!'
twitter_title: 'AWS Copilot v1.20'
image: 'https://user-images.githubusercontent.com/879348/179278910-1e1ae7e7-cb57-46ff-a11c-07919f485c79.png'
image_alt: 'Environment manifests'
image_width: '1106'
image_height: '851'
---

# AWS Copilot v1.20: To Environment Manifests and Beyond!

Posted On: Jul 19, 2022

The AWS Copilot core team is announcing the Copilot v1.20 release.  
Special thanks to [@gautam-nutalapati](https://github.com/gautam-nutalapati), [@codekitchen](https://github.com/codekitchen), and [@kangere](https://github.com/kangere/) who contributed to this release.
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has over 300 people online and over 2.3k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.20 brings several new features and improvements:

* **Environment manifests**: You can now create and update environments with a [manifest file](../docs/manifest/environment.en.md) bringing all the benefits of infrastructure as code to environments.
   [See detailed walkthrough](#environment-manifest) for how to migrate your existing environments.
* **Autoscaling Cooldown Support**: You can now specify [autoscaling cooldowns](#autoscaling-cooldown-support) in the service manifest.
* **Additional policy to build role**: You can now specify an additional policy for the CodeBuild Build Project Role through the pipeline manifest field `additional_policy`.
  [See detailed walkthrough](../docs/manifest/pipeline.en.md) for how to specify an additional policy document to add to the build project role. [(#3709)](https://github.com/aws/copilot-cli/pull/3709)
* **Invoke a scheduled job**: You can now execute an existing scheduled job ad hoc using the new `copilot job run` command.
  [(#3692)](https://github.com/aws/copilot-cli/pull/3692)
* **Deny default security group**: Add an option `deny_default` to `security_groups` in service manifests to remove the EnvironmentSecurityGroup ingress that is applied by default.
  [(#3682)](https://github.com/aws/copilot-cli/pull/3682)
* **Predictable aliases for Backend Services with an ALB**: If you don't specify an alias for your Backend Services that have an internal ALB configured, they will now be reachable with the host name `svc.env.app.internal` instead of the default ALB host name. ([#3668](https://github.com/aws/copilot-cli/pull/3668))

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.  
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Environment Manifest

Prior to v1.20, clients could not update their environments with additional configuration. For example, if your environment wasn't associated
with a domain, users would need to run `env init --name copy --import-cert-arns` to create a new environment with certificates and then tear down the old one.
Starting with this release, users can modify their environments using the [manifest](../docs/manifest/environment.en.md) without having
to recreate them.  
Moving forward new environment resources will be configured with the `manifest.yml` file instead of flags in the `env init` command.

### Walkthrough
**[1\]** `copilot env init` **no longer** immediately deploys an environment in your account. Instead, the command
writes a [manifest.yml](../docs/manifest/environment.en.md) file to your local workspace.

??? example "Running `copilot env init`"

    ```console
    $ copilot env init
    Environment name: prod-pdx
    Credential source: [profile default]
    Default environment configuration? Yes, use default.
    ✔ Wrote the manifest for environment prod-pdx at copilot/environments/prod-pdx/manifest.yml
    ...additional output messages
    ```

    ```console
    $ cat copilot/environments/prod-pdx/manifest.yml
    # The manifest for the "prod-pdx" environment.
    # Read the full specification for the "Environment" type at:
    #  https://aws.github.io/copilot-cli/docs/manifest/environment/

    # Your environment name will be used in naming your resources like VPC, cluster, etc.
    name: prod-pdx
    type: Environment

    # Import your own VPC and subnets or configure how they should be created.
    # network:
    #   vpc:
    #     id:

    # Configure the load balancers in your environment, once created.
    # http:
    #   public:
    #   private:

    # Configure observability for your environment resources.
    observability:
      container_insights: false
    ```

**[2\]** After modifying the manifest, you can run the new `copilot env deploy` command to create or update
the environment stack.

??? example "Running `copilot env deploy`"

    ```console
    $ copilot env deploy
    Name: prod-pdx
    ✔ Proposing infrastructure changes for the demo-prod-pdx environment.
    - Creating the infrastructure for the demo-prod-pdx environment.              [update complete]  [110.6s]
      - An ECS cluster to group your services                                     [create complete]  [9.1s]
      - A security group to allow your containers to talk to each other           [create complete]  [6.3s]
      - An Internet Gateway to connect to the public internet                     [create complete]  [18.5s]
      - Private subnet 1 for resources with no internet access                    [create complete]  [6.3s]
      - Private subnet 2 for resources with no internet access                    [create complete]  [6.3s]
      - A custom route table that directs network traffic for the public subnets  [create complete]  [15.5s]
      - Public subnet 1 for resources that can access the internet                [create complete]  [6.3s]
      - Public subnet 2 for resources that can access the internet                [create complete]  [6.3s]
      - A private DNS namespace for discovering services within the environment   [create complete]  [47.2s]
      - A Virtual Private Cloud to control networking of your AWS resources       [create complete]  [43.6s]
    ```

And that's it 🚀! The workflow is identical to how `copilot svc` and `copilot job` commands work.

### Migrating Existing Environments

To create a [manifest.yml](../docs/manifest/environment.en.md) file for existing environments, Copilot
introduced a new `--manifest` flag to `copilot env show`.  
In this example, we'll generate a manifest file for an existing `"prod"` environment.

**[1\]** First, in your current git repository or in a new one, create the mandatory directory
structure for environment manifests.

???+ example "Directory structure for prod"

    ```console
    # 1. Navigate to your git repository.
    $ cd my-sample-repo/
    # 2. Create the directory for the "prod" environment  
    $ mkdir -p copilot/environments/prod
    ```

**[2\]** Run the `copilot env show --manifest` command to generate a manifest and redirect it to the "prod" folder.

???+ example "Generate manifest"

    ```console
    $ copilot env show -n prod --manifest > copilot/environments/prod/manifest.yml
    ```

And that's it! you can now modify the manifest file with any fields in the [specification](../docs/manifest/environment.en.md) and
run `copilot env deploy` to update your stack.

### Continuous Delivery

Finally, Copilot provides the same [continuous delivery pipeline](../docs/concepts/pipelines.en.md) workflow for environments as
services or jobs.

**[1\]** Once a [manifest file is created](#migrating-existing-environments), you can run the existing `copilot pipeline init`
command to create a pipeline [`manifest.yml`](../docs/manifest/pipeline.en.md) file to describe deployment stages, and a
`buildspec.yml` used in the "Build" stage to generate the CloudFormation configuration files.

??? example "Create pipeline manifest and buildspec"

    ```console
    $ copilot pipeline init                
    Pipeline name: env-pipeline
    What type of continuous delivery pipeline is this? Environments
    1st stage: test
    2nd stage: prod

    ✔ Wrote the pipeline manifest for copilot-pipeline-test at 'copilot/pipelines/env-pipeline/manifest.yml'    
    ✔ Wrote the buildspec for the pipeline's build stage at 'copilot/pipelines/env-pipeline/buildspec.yml'
    ```

**[2\]** Run `copilot pipeline deploy` to create or update the AWS CodePipeline stack.

??? example "Create pipeline"

    ```console
    $ copilot pipeline deploy                                                 
    Are you sure you want to redeploy an existing pipeline: env-pipeline? Yes
    ✔ Successfully deployed pipeline: env-pipeline
    ```

## Autoscaling Cooldown Support
A small addition to our service manifests: the ability to configure autoscaling cooldown periods.
For `Load Balanced`, `Backend`, and `Worker` Services, you can now configure their autoscaling fields under `count` to have custom cooldown periods.
Previously, each scaling metric such as `cpu_percentage` had a fixed 'in' cooldown of 120 secs and 'out' cooldown of 60 seconds. Now, you can set a global cooldown period:

??? example "Using general autoscaling cooldowns"

    ```
    count:
      range: 1-10
      cooldown:
        in: 30s
        out: 30s
      cpu_percentage: 50
    ```

Alternatively, you can set individual cooldowns that override the general ones:

??? example "Using specific autoscaling cooldowns"

    ```
    count:
      range: 1-10
      cooldown:
        in: 2m
        out: 2m
      cpu_percentage: 50
      requests:
        value: 10
        cooldown:
          in: 30s
          out: 30s
    ```
## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

* Download [the latest CLI version](../docs/getting-started/install.en.md)
* Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
* Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.20.0)
