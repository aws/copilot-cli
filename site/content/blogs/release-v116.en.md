# AWS Copilot v1.16: Multiple pipelines, SNS subscription filters and more

Posted On: Apr 6, 2022

The AWS Copilot core team is excited to announce the Copilot v1.16 release.
We welcome 7 new community developers who contributed to this release: [@codekitchen](https://github.com/codekitchen),
[@shingos](https://github.com/shingos), [@csantos](https://github.com/csantos), [@rfma23](https://github.com/rfma23),
[@g-grass](https://github.com/g-grass), [@isleys](https://github.com/isleys),
[@kangere](https://github.com/kangere). Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has over 270 people online,
who help each other daily and we recently passed a milestone of over 2.1k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.16 brings with it several new features and improvements:

* **Multiple pipelines:** You can now run `copilot pipeline init` to create multiple CodePipelines that track separate
branches in your repository. [See detailed section](./#create-multiple-pipelines-per-branch).
* **SNS subscription filter policies:** Worker services can now filter SNS messages for each subscribed topic
using the `filter_policy` field. [See detailed section](./#define-messages-filter-policy-in-publishsubscribe-service-architecture).
* **Lots of other improvements:**
    * Add a `--no-rollback` flag to the `deploy` commands to disable automatic stack rollback in case of a deployment failure ([#3341](https://github.com/aws/copilot-cli/pull/3341)). The new flag is useful for troubleshooting infrastructure change failures. For example, if a deployment fails CloudFormation will delete the logs that happened during the failure because it rolls back the stack. This flag will help preserve the logs to troubleshoot the issue, then you can rollback and update your manifest again.
    * Add a `--upload-assets` flag to the `package` commands to push assets to ECR or S3 before generating CloudFormation templates ([#3268](https://github.com/aws/copilot-cli/pull/3268)). Your pipeline buildspec can now be significantly simplified with this flag. If you'd like to regenerate the buildspec, delete the file and run `copilot pipeline init` again, using your existing pipeline name at the prompt.
    * Allow additional security groups when running `task run` in an environment ([#3365](https://github.com/aws/copilot-cli/pull/3365)).
    * Make Docker progress updates less noisy when Copilot is running in CI environment (the environment variable `CI` is set to `true`) ([#3345](https://github.com/aws/copilot-cli/pull/3345)).
    * Log a warning when deploying an App Runner service in a region where it's not available yet ([#3326](https://github.com/aws/copilot-cli/pull/3326)).
    * `app show` and `env show` commands now display the deployed environments for services and jobs in a table format ([#3379](https://github.com/aws/copilot-cli/pull/3379), [#3316](https://github.com/aws/copilot-cli/pull/3316)).
    * Customize buildspec path in the pipeline manifest with `build.buildspec` ([#3403](https://github.com/aws/copilot-cli/pull/3403)).

## What’s AWS Copilot?

The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.  
From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code in a single operation.
Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro services creation and operation,
enabling you to focus on developing your application, instead of writing deployment scripts.

See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Create Multiple Pipelines per Branch
_Contributed by [Efe Karakus](https://github.com/efekarakus/), [Janice Huang](https://github.com/huanjani/)_

Having an automated release process is one of the most important parts of software delivery, so AWS Copilot wants to make setting up that process as easy as possible.
Instead of running `copilot deploy` manually for all the environments in your application,
you can run just a few `copilot pipeline` commands to setup a continuous delivery pipeline that automatically releases to the environments whenever you `git push`.

The generated CodePipeline has the following basic structure:

* Source stage: when you push to a configured GitHub, Bitbucket, or CodeCommit repository branch, a new pipeline execution is triggered.
* Build: After your source code is pulled from your repository host, your service's container image is built and published to every environment's ECR repository and any input files, such as addons templates, lambda function zip files, and environment variable files, are uploaded to S3
* Deploy stages: After your code is built, you can deploy to any or all of your environments, with optional post-deployment tests or manual approvals.

Previously, Copilot allowed creation of only a single pipeline per git repository. Running `copilot pipeline init` resulted in a single pipeline manifest file; for example, the manifest file below models a CodePipeline that first releases to “test,” then once the deployment succeeds, to the “prod” environment:

```
$ copilot pipeline init
1st stage: test
2nd stage: prod
✔ Wrote the pipeline manifest for my-pipeline at 'copilot/pipeline.yml'

Required follow-up actions:
- Commit and push the buildspec.yml, pipeline.yml, and .workspace files of your copilot directory to your repository.
- Run `copilot pipeline deploy` to create your pipeline.

$ cat copilot/pipeline.yml
name: my-pipeline
source:
  provider: GitHub
  properties:
    branch: main
    repository: https://github.com/user/repo
stages:
    - name: test
    - name: prod
    # requires_approval: true
    # test_commands: [echo 'running tests', make test]
```

This model works well for users that want every commit to the “main” branch to be released across their environments.
An alternative model is to create a pipeline per branch. For example, I could commit several changes to “main” branch, then, once satisfied, merge the changes to the “test” branch to deploy to the “test” environment, and then merge to the “prod” branch. Until v1.16, this model was not possible because only a single pipeline manifest was supported.

Starting with v1.16, Copilot users can now create multiple pipelines in their git repository so that they can have a separate pipeline per branch. For example, I can run `copilot pipeline init` in separate branches of my git repository without worrying about merge conflicts:

```
$ git checkout test
$ copilot pipeline init
Pipeline name: repo-test
1st stage: test
Your pipeline will follow branch 'test'.

✔ Wrote the pipeline manifest for repo-test at 'copilot/pipelines/repo-test/manifest.yml'
Required follow-up actions:
- Commit and push the copilot/ directory to your repository.
- Run `copilot pipeline deploy` to create your pipeline.

$ git checkout prod
$ copilot pipeline init
Pipeline name: repo-prod
1st stage: prod
Your pipeline will follow branch 'prod'.

✔ Wrote the pipeline manifest for repo-prod at 'copilot/pipelines/repo-prod/manifest.yml'
Required follow-up actions:
- Commit and push the copilot/ directory to your repository.
- Run `copilot pipeline deploy` to create your pipeline.
```

Once changes are merged to the “test” branch, then the “repo-test” pipeline will be triggered. Similarly, I can then promote the changes to the “prod” branch and trigger the “repo-prod” pipeline.

You can learn more about pipelines in [Copilot's docs](../docs/concepts/pipelines.en.md).

## Define messages filter policy in Publish/Subscribe service architecture
_Contributed by [Penghao He](https://github.com/iamhopaul123/)_

A common need in microservices architecture is to have an easy way to implement the robust publish/subscribe logic for passing messages between services.
AWS Copilot leverages a combination of Amazon SNS and Amazon SQS services to make it easy.
With AWS Copilot, you can configure single or multiple services to publish messages to named SNS topics and create worker services to receive and process those messages.
AWS Copilot configures and auto-provisions required pub/sub infrastructure, including SNS topics, SQS queues, and required policies.
You can read more about [Publish/Subscribe architecture in AWS Copilot documentation](../docs/developing/publish-subscribe.en.md).

By default, an Amazon SNS topic subscriber receives every message published to the topic.
To receive a subset of the messages, a subscriber must assign a filter policy to the topic subscription. For example, you might have a service that publishes orders to a topic:

```yaml
`# manifest.yml for api service
name: api
type: Backend Service
publish:
  topics:
    - name: ordersTopic`
```

and a worker that processes all type of messages in `ordersTopic`:

```yaml
name: orders-worker
type: Worker Service

subscribe:
  topics:
    - name: ordersTopic
      service: api
  queue:
    dead_letter:
      tries: 5
```

AWS Copilot will create a subscription and provision all required infrastructure, so you can focus on writing your code.
However, let’s say you need to create a new worker that only processes messages of canceled orders with price value of more than $100.

Starting with the Copilot v1.16 release, you don’t need to filter out those messages in your code; you can simply define a SNS subscription filter policy:

```yaml
name: orders-worker
type: Worker Service

subscribe:
  topics:
    - name: ordersTopic
      service: api
      filter_policy:
        store:
            - example_corp
        event:
            - order_canceled
        price_usd:
            - numeric:
              - ">="
              - 100
  queue:
    dead_letter:
      tries: 5
```

With this filter policy in place, Amazon SNS will filter all messages by matching those attributes.
You can learn more about SNS filters in [Copilot's documentation](../docs/manifest/worker-service.en.md#topic-filter-policy).

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

* Download [the latest CLI version](../docs/getting-started/install.en.md)
* Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
* Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.16.0)
