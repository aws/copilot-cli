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
	However, each user has their own unique situation. If you do need your data storage to be shared among multiple services,
	you can modify the Copilot-generated CloudFormation template in order to achieve your goal.

Here is an example of prompts that you might see.

!!! note ""
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
under your "copilot/environments" directory. In addition, it will generate the necessary access policy; here is one that grants "api" service 
access to the "movies" storage. The access policy is created as a workload addon, meaning that it is deployed and
deleted at the same time as the "api" service.

!!! note ""
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

Depending on the storage type, and the type of the workload that is fronting the storage, Copilot may generate different
CloudFormation files.

???- note "Sample Files generated for an Aurora Serverless fronted by a Request-Driven Web Service"
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

## Sidecar image build

## Request-Driven Web Service secret support

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0)
