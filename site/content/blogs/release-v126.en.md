---
title: 'AWS Copilot v1.26: Automate rollbacks with CloudWatch alarms, `storage init` for env addons, and RDWS secrets support'
twitter_title: 'AWS Copilot v1.26'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.26: Automate rollbacks with CloudWatch alarms, `storage init` for env addons, and RDWS secrets support

Posted On: Feb 20, 2023

The AWS Copilot core team is announcing the Copilot v1.26 release.  
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has over 400 people online and over 2.6k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.26 brings several new features and improvements:

- **Service alarm-based rollback**: [See detailed section](#service-alarm-based-rollback).
- **`storage init` for environment addons**: [See detailed section](#storage-init-for-environment-addons).
- **Request-Driven Web Service secrets support**: [See detailed section](#request-driven-web-service-secrets-support).

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready containerized applications on AWS.
    From getting started to releasing in production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application instead of writing deployment scripts.

    See the [Overview](../docs/concepts/overview.en.md) section for a more detailed introduction to AWS Copilot.

## Service alarm-based rollback
You can now [monitor your ECS deployments](https://aws.amazon.com/blogs/containers/automate-rollbacks-for-amazon-ecs-rolling-deployments-with-cloudwatch-alarms/) with custom [CloudWatch alarms](https://docs.aws.amazon.com/AmazonECS/latest/userguide/deployment-alarm-failure.html)! Configure your services to roll back to the last completed deployment if your alarms go into `In alarm` state during deployment. With the [circuit breaker](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-circuit-breaker.html), Copilot has already been rolling back your failed deployments. Now, you can also roll back service deployments that aren't failing, but aren't performing in accordance with the metrics of your choice.

In your backend, worker, or load-balanced web service manifest, you may import your own existing CloudWatch alarms:
    ```yaml
    deployment:
      rollback_alarms: ["MyAlarm-ELB-4xx", "MyAlarm-ELB-5xx"]
    ```

Or have Copilot create a CPU and/or memory utilization alarm for you, with thresholds of your choice:
    ```yaml
    deployment:
      rollback_alarms:
        cpu_utilization: 70    // Percentage value at or above which alarm is triggered.
        memory_utilization: 50 // Percentage value at or above which alarm is triggered.
    ```

For worker services, you may also create an alarm to monitor `ApproximateNumberOfMessagesDelayed`:
    ```yaml
    deployment:
      rollback_alarms:
        messages_delayed: 5
    ```

 When Copilot creates alarms for you, some defaults are set under the hood:
    ```yaml
    ComparisonOperator: 'GreaterThanOrEqualToThreshold'
    DatapointsToAlarm: 2
    EvaluationPeriods: 3
    Period: 60
    Statistic: 'Average'
    ```
With rollback alarms configured in your service manifest, each time you run `svc deploy` after the initial deployment (when there is no existing deployment to roll back to), ECS will poll your alarms and trigger a rollback if there is a breach.

## `storage init` for environment addons

Previously, `copilot storage init` only supported storage addons attached to workloads: you need to run
`copilot svc deploy` in order to deploy the storage, and the storage is deleted along with the service
when you run `copilot svc delete`.

Now, you have the option to create environment storage addons: the storage is deployed when you run `copilot env deploy`,
and isn't deleted until you delete the environment by running `copilot env delete`.

Similar to the workload storage, the environment storage is, under the hood, just another [environment addon](../docs/developing/addons/environment.en.md)!

### [Database-Per-Service](https://docs.aws.amazon.com/prescriptive-guidance/latest/modernization-data-persistence/database-per-service.html) By Default
In the microservice world, it is generally recommended to set up data storage resources that are each private to a microservice,
instead of monolith storages that are shared by multiple services.
This pattern preserves the core characteristics of microservices - loose coupling.
Copilot encourages you to follow this database-per-service pattern. By default, a storage resource that Copilot generates
is assumed to be accessed by one service or job.

!!!note ""
    However, each user has its own unique situation. If you do need your data storage to be shared among multiple service,
    you can modify the Copilot-generated CloudFormation template in order to achieve your goal.


Here is an example of prompts that you might see.
!!! info ""

    ```term
    $ copilot storage init
    What type of storage would you like to create?
    > DynamoDB            (NoSQL)
      S3                  (Objects)
      Aurora Serverless   (SQL)

    Which workload needs access to the storage?
    > api
      backend

    What would you like to name this DynamoDB Table? movies

    Do you want the storage to be created and deleted with the api service?
      Yes, the storage should be created and deleted at the same time as api
    > No, the storage should be created and deleted at the environment level
    ```
You can skip the prompts using the flags. The following command is equivalent to the prompts above:
```console
copilot storage init \
--storage-type "DynamoDB" \
--workload "api" \
--name "movies" \
--lifecycle "environment"
```


After you've answered all the prompts or skipped them by using flags, Copilot will generate the CloudFormation template that defines the DynamoDB storage resource
under your `copilot/environments` directory. In addition, it will generate the necessary access policy; here is one that grants "api" service 
access to the "movies" storage. The access policy is created as a workload addon, meaning that it is deployed and
deleted at the same time as the "api" service.
!!! info ""
    ```
    copilot/
    ├── environments/
    │   ├── addons/         
    │   │   └── movies.yml                # <- The CloudFormation template that defines the "movies" DynamoDB storage.
    │   └── test/
    │       └── manifest.yml
    └── api
        ├── addons/
        │   └── movies-access-policy.yml  # <- The CloudFormation template that defines the access policy.
        └─── manifest.yml
    ```

Depending on the storage type, and the type of the workload that is fronting the storage, Copilot may generate different
CloudFormation files.

???- note "Sample files generated for an Aurora Serverless fronted by a Request-Driven Web Service"
    ```
    # Example: an Aurora Serverless v2 storage whose lifecycle is at the environment-level, faced by a Request-Driven Web Service.
    copilot/
    ├── environments/
    │   └── addons/
    │         ├── addons.parameters.yml   # The extra parameters required by the Aurora Serverless v2 storage.
    │         └── user.yml                # An Aurora Serverless v2 storage
    └── api                               # "api" is a Request-Driven Web Service
        └── addons/
            ├── addons.parameters.yml   # The extra parameters required by the ingress resource.
            └── user-ingress.yml        # A security group ingress that grants "api" access to the "api" storage"
    ```



Also check out our [storage page](../docs/developing/storage.en.md) for more information!


## Request-Driven Web Service secrets support
You can now add your secrets (from SSM Parameter Store or AWS Secrets Manager) to your App Runner service as environment variables using Copilot.

Similar to other service types such as Load-Balanced Web Service, you need to first add the following tags to your secrets:

| Key                     | Value                                                       |
| ----------------------- | ----------------------------------------------------------- |
| `copilot-application`   | Application name from which you want to access the secret   |
| `copilot-environment`   | Environment name from which you want to access the secret   |

Then simply update your Request-Driven Web Service manifest with:
```yaml
  secrets:
    GITHUB_TOKEN: GH_TOKEN_SECRET
```
And deploy! Your service can now access the secret as an environment variable.

For more detailed use of the `secrets` field:
```yaml
secrets:
  # To inject a secret from SecretsManager.
  # (Recommended) Option 1. Referring to the secret by name.
  DB:
    secretsmanager: 'demo/test/mysql'
  # You can refer to a specific key in the JSON blob.
  DB_PASSWORD:
    secretsmanager: 'demo/test/mysql:password::'
  # You can substitute predefined environment variables to keep your manifest succinct.
  DB_PASSWORD:
    secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql:password::'

  # Option 2. Alternatively, you can refer to the secret by ARN.
  DB: 'arn:aws:secretsmanager:us-west-2:111122223333:secret:demo/test/mysql-Yi6mvL'

  # To inject a secret from SSM Parameter Store
  # Option 1. Referring to the secret by ARN.
  GITHUB_WEBHOOK_SECRET: 'arn:aws:ssm:us-east-1:615525334900:parameter/GH_WEBHOOK_SECRET'

  # Option 2. Referring to the secret by name.
  GITHUB_WEBHOOK_SECRET: GITHUB_WEBHOOK_SECRET
```
See the [manifest specification](../docs/manifest/rd-web-service/#secrets). To learn more about injecting secrets into your services, see [the secrets page](../docs/developing/secrets.en.md)

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0)
