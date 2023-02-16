---
title: 'AWS Copilot v1.26: Automate rollbacks with CloudWatch alarms, build sidecar images, and `storage init` for env addons'
twitter_title: 'AWS Copilot v1.26'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.26: Automate rollbacks with CloudWatch alarms, build sidecar images, and `storage init` for env addons

Posted On: Feb 20, 2023

The AWS Copilot core team is announcing the Copilot v1.26 release.  
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has almost 400 people online and over 2.6k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.26 brings several new features and improvements:

- **Service alarm-based rollback**: [See detailed section](#service-alarm-based-rollback).
- **`storage init` for environment addons**: [See detailed section](#storage-init-for-environment-addons).
- **Sidecar image build**: [See detailed section](#sidecar-image-build).
- **Request-Driven Web Service secret support**: [See detailed section](#request-driven-web-service-secret-support).

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Service alarm-based rollback

## `storage init` for environment addons

Previously, `copilot storage init` only supports storage addon that is attached to your workload: 
you need to run `copilot svc deploy` in order to deploy the storage, and the storage is deleted along with the service
when you run `copilot svc delete`.

Now, you have the option to create an environment-level storage addon: the storage will be deployed as you run `copilot env deploy`,
and won't be deleted until you delete the whole environment by running `copilot env delete`.

Similar to the workload-level storage, the environment-level storage is under the hood just another [environment addon](../docs/developing/addons/environment.en.md)!

### Best Practice By Default
Following the best practice in the microservice world, Copilot encourages you to set up storages that are each accessible
by only one service, instead of monolith storages that are shared by all microservices. Therefore, Copilot assumes
that your storage is designed to be accessed by one of your services or jobs, even when it is meant to live and die
with the environment. Here is an example of prompts that you might see:

```console
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

After you've answered all the necessary prompts, Copilot will generate the CloudFormation template that defines the DynamoDB storage
under your `copilot/environments` directory. In addition, it will generate the access policy that grants "api" service 
access to the "movies" storage:

```
copilot/
├── environments/
│   ├── addons/         
│   │     └── movies.yml                # <- The CloudFormation template that defines the "movies" DynamoDB storage.
│   └── test/
│         └── manifest.yml
└── api
    ├── addons/
    │     └── movies-access-policy.yml  # <- The CloudFormation template that defines the access policy.
    └─── manifest.yml
```

The access policy is created as a workload-level addon that lives and dies with your service.


Depending on the storage type, and the type of the workload that is facing the storage, Copilot may generate different
addon files.

???- note "Sample Files"
	```
	# Example: an environment-level Aurora Serverless v2 storage, faced by a Request-Driven Web Service.
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

At the same time, Copilot will print out a series of recommended actions on your terminal to help you finish the deployment. For example,
```console
Recommended follow-up actions:
  - Run `copilot env deploy` to deploy your environment storage resources.
  - Update the manifest for your "api" workload:
    ```
    variables:
      DB_NAME:
        from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-moviesTableName
    ```
  - Run `copilot svc deploy --name api` to deploy the workload so that api has access to movies storage.
```

Also check out our [storage page](../docs/developing/storage.en.md) for more information!

## Sidecar image build

## Request-Driven Web Service secret support

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0)
