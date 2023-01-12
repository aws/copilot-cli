---
title: 'AWS Copilot v1.25: Environment addons and static content delivery.'
twitter_title: 'AWS Copilot v1.25'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.25: Environment addons and static content delivery.

Posted On: Jan 17, 2023

The AWS Copilot core team is announcing the Copilot v1.25 release.  
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has almost 400 people online and over 2.6k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.25 brings several new features and improvements:

- **Environment Addons**: [See detailed section](#environment-addons).
- **Static Content Delivery With CloudFront**: [See detailed section](#static-content-delivery-with-cloudfront).

???+ note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
    From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro service architectures,
    enabling you to focus on developing your application, instead of writing deployment scripts.

    See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Environment Addons

You can now deploy addons associated with your environments.

Addons are additional AWS resources that are not integrated in Copilot by default - for example, DynamoDB, RDS, etc. 
Environment addons are additional resources managed on the environment level. Their lifecycle will be tied
to your environment: you will run `copilot env deploy` to create or update your environment addons; 
when you run `copilot env delete` against an environment, Copilot will try to delete its addons as well.

If you are already familiar with workload addons, then good news - 
the experience of managing environment addons is pretty similar.

#### Getting Started
##### Step 1: Model additional AWS resources with CloudFormation
Today, addons only support modelling using CloudFormation. For environment addons, you must:

1. Have `App` and `Env` in the `Parameters` section.
2. Include at least one `Resource`.

[Here](TODO: link) is an example CloudFormation template that you can use to experiment.

##### Step 2: Store the CFN template under `copilot/environments/addons`

If you have run `copilot env init`, you should already have the folder `copilot/environments` in your workspace.
If you haven't, it's recommended to do so now - you will need it sooner or later.

At the end, your workspace structure may look like this:
```
copilot/
├── environments/ 
│   ├── addons/  
│   │     ├── appmesh.yml         
│   │     └── ddb.yml      # <- You can have multiple addons.  
│   ├── test/ 
│   │     └─── manifest.yml 
│   └── dev/ 
│         └── manifest.yml
└── web
    ├── addons/
    │     └── s3.yml       # <- A workload addon template.
    └─── manifest.yml 
```

##### Step 3: Run `copilot env deploy`

After you run `copilot env deploy`, Copilot will scan through the `addons` folder to look for addons template. 
If it is able to find any, it will deploy the templates along with the environment. 

##### (Optional) Step 4: Verify the deployment

You can verify the deployment by going to the [AWS CloudFormation console](https://us-east-1.console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks) 
in your region. You should be able to find a stack named `[app]-[env]-AddonsStack-[random string]`. This is a nested stack
created under your environment stack, named `[app]-[env]`.

### Feature Parity With Workload Addons
Environment addons is shipped with all existing features available for workload addons. This means that:

1. You can refer to customized `Parameters` in addition to the required `App` and `Env`.
2. In your templates, you can reference local paths. Copilot will upload those local files and replace the relevant
resource properties with the uploaded S3 location.

Read [here](TODO: link) for more!

### Other Considerations
All environments (in the example above, both "test" and "dev") will share the same addon templates. 
Just like today’s workload-level addons, any environment-specific configuration should be specified inside the 
addon templates via the `Conditions` and `Mappings` sections. 
This is designed to follow  [CFN’s best practice of reusing templates](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/best-practices.html#reuse).

```yaml
AWSTemplateFormatVersion: 2010-09-09
Parameters:
  App:
    Type: String
    Description: Your application name.
  Env:
    Type: String
    Description: The name of the environment being deployed.

Conditions:
  IsTestEnv: !Equals [ !Ref Env, "test" ]  # <- Use `Conditions` section to specify "test"-specific configurations.

Mappings:
  ScalingConfigurationMapByEnv:
    test:
      "DBMinCapacity": 0.5
    prod:
      "DBMinCapacity": 1
```

### Integrating With Workloads
todo @wanxiay

### Import Values From CloudFormation Stacks In Workload Manifests

## Static Content Delivery With CloudFront

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0)