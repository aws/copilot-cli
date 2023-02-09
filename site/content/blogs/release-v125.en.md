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
Today, addons only support modeling using CloudFormation. For environment addons, you must:

1. Have `App` and `Env` in the `Parameters` section.
2. Include at least one `Resource`.

???- note "Sample CloudFormation template"
    Here is a CloudFormation template example that you can experiment with.

    ```yaml
    AWSTemplateFormatVersion: 2010-09-09
    Parameters:
      App:
        Type: String
        Description: Your application's name.
      Env:
        Type: String
        Description: The name of the environment being deployed.
    Resources:
      MyTable:
        Type: 'AWS::DynamoDB::Table'
        Properties:
          TableName: MyEnvAddonsGettingStartedTable
          AttributeDefinitions:
            - AttributeName: key
              AttributeType: S
          KeySchema:
            - AttributeName: key
              KeyType: HASH
          ProvisionedThroughput:
            ReadCapacityUnits: 5
            WriteCapacityUnits: 2
    Outputs:
      MyTableARN:
        Value: !GetAtt MyTable.Arn
        Export:
          Name: !Sub ${App}-${Env}-MyTableARN
      MyTableName:
        Value: !Ref MyTable
        Export:
          Name: !Sub ${App}-${Env}-MyTableName
    ```

##### Step 2: Store the CFN template under `copilot/environments/addons`

If you have run `copilot env init`, you should already have the folder `copilot/environments` in your workspace.
If you haven't, it's recommended to do so now - you will need it sooner or later.

Afterward, your workspace structure may look like this:
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

After you run `copilot env deploy`, Copilot will scan through the `addons` folder to look for addons templates.
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

Read [here](../docs/developing/addons/workload.en.md) for more!

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
You can reference values from your environment addons in your workload-level resources.

#### Reference Environment Addons Values In Workload Addon

##### Step 1: Export values from your environment addons
In an environment addon template, you should add an `Outputs` section, and define the `Output` that you want your
workload resource to reference. See [this doc](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html) for CloudFormation `Outputs` syntax.

Taking the example template that we provided - this is the `Outputs` section that we have added in the example.
```yaml
Outputs:
  MyTableARN:
    Value: !GetAtt MyTable.Arn
    Export:
      Name: !Sub ${App}-${Env}-MyTableARN
  MyTableName:
    Value: !Ref MyTable
    Export:
      Name: !Sub ${App}-${Env}-MyTableName
```

You can specify any name you like for `Export.Name`. However, the name must be unique within an AWS region; therefore,
we recommend that you namespace it with `${App}` and `${Env}` to reduce the chances of name collision.
With the namespace, for example, say your application's name  is `"my-app"`,
and you deployed the addons with environment `test`, then the final export name would be `my-app-test-MyTableName`.

After you've made the code change, run `copilot env deploy` for the change to take effect.


##### Step 2: Import the values from your workload addons.

In your workload addons, use the [`Fn::ImportValue`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/intrinsic-function-reference-importvalue.html) function to import the value that you just exported from your environment addons.

Continuing the above example, say I now want my `db-front` service to access `MyTable`. I will create a workload addon
attached to `db-front` with an IAM policy that gives it access.

```yaml
Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.
  Name:
    Type: String
    Description: The name of the service, job, or workflow being deployed.
Resources:
  MyTableAccessPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      Description: Grants CRUD access to the Dynamo DB table
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
          - Sid: DDBActions
            Effect: Allow
            Action:
              - dynamodb:* # NOTE: Scope down the permissions in your real application. This is done so that this blog post isn't too long!
            Resource:
              Fn::ImportValue:                # <- We import the table ARN from the environment addons.
                !Sub ${App}-${Env}-MyTableARN # <- The export name that we used.
```

For another example, suppose you did not namespace your `Export.Name`, and instead gave your export a name like this:
```yaml
Outputs:
  MyTableARN:
    Value: !GetAtt MyTable.Arn
    Export:
      Name: !Sub MyTableARN
```

You should instead import this value with
```yaml
Fn::ImportValue:       
  !Sub MyTableARN
```

This is how you would hook your workload addons with your environment addons!

#### Reference Environment Addons Values In Workload Manifests

If you need to reference any value from your environment addons - for example, adding a secret created in an environment addon
to your service - you can use the feature [`from_cfn` in workload manifests](#import-values-from-cloudformation-stacks-in-workload-manifests)
to do so.

##### Step 1: Export values from your environment addons
Same as when working with workload addons, you need to export the value from your environment addons.

```yaml
Outputs:
  MyTableName:
    Value: !Ref MyTable
    Export:
      Name: !Sub ${App}-${Env}-MyTableName
```

##### Step 2: Reference the value using `from_cfn` in your workload manifests
Suppose I want to inject the table name as an environment variable in my `db-front` service, then my `db-front` service
should have a manifest that looks like
```yaml
name: db-front
type: Backend Service

// Other configurations...

variables:
  MY_TABLE_NAME:
    from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyTableName
```

Similarly, if you have exported your table name without the namespace like this:
```yaml
Outputs:
  MyTableName:
    Value: !Ref MyTable
    Export:
      Name: MyTableName
```

Then your manifest should have instead
```yaml
variables:
  MY_TABLE_NAME:
    from_cfn: MyTableName
```

### Import Values From CloudFormation Stacks In Workload Manifests

You can now import values from environment addons' CloudFormation stacks or any other stack in your workload manifest using `from_cfn`.
To reference a value from another CloudFormation stack, users should first export the output value from the source stack.

Here is an example of how the `Outputs` section of a CloudFormation template looks when exporting values from other stacks or creating cross-stack references.

```yaml
Outputs:
  WebBucketURL:
    Description: URL for the website bucket
    Value: !GetAtt WebBucket.WebsiteURL
    Export:
      Name: stack-WebsiteUrl # <- Unique export name within the region.
```

To find our more, see [this page](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html).

For now, `from_cfn` is added only to the following workload manifest fields.

```yaml
variables:
  LOG_LEVEL: info
  WebsiteUrl:
    from_cfn: stack-WebsiteUrl
```

```yaml
secrets:
  GIT_USERNAME:
    from_cfn: stack-SSMGHUserName
```

```yaml
logging:
  secretOptions:
    GIT_USERNAME:
      from_cfn: stack-SSMGHUserName
```

```yaml
sidecars:
  secrets:
    GIT_USERNAME:
      from_cfn: stack-SSMGHUserName
```

```yaml
network:
  vpc:
    security_groups:
      - sg-1234
      - from_cfn: UserDBAccessSecurityGroup
```

## Static Content Delivery With CloudFront
You can now bring your own S3 bucket to work with CloudFront for faster static content delivery. More native support for bucket management (for example, bucket creation and asset upload) will be included in future releases.

### (Optional) Create an S3 bucket
If you don't have an existing S3 bucket, use the S3 console/AWS CLI/SDK to create an S3 bucket. Note that for security concerns, we strongly recommend creating a private S3 bucket, which blocks public access by default.

### Configuring CloudFront in the env manifest
You can use CloudFront with an S3 bucket as the origin by configuring the environment manifest as below:

```yaml
cdn:
  static_assets:
    location: cf-s3-ecs-demo-bucket.s3.us-west-2.amazonaws.com
    alias: example.com
    path: static/*
```

More specifically, `location` is the [DNS domain name of the S3 bucket](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/distribution-web-values-specify.html#DownloadDistValuesDomainName), and the static assets will be accessible at `example.com/static/*`.

### (Optional) Update bucket policy
If the bucket you use for CloudFront is **private**, you need to update the bucket policy to grant read access to CloudFront. To use the example above, we need to update the bucket policy for `cf-s3-ecs-demo-bucket` to

```json
{
    "Version": "2012-10-17",
    "Statement": {
        "Sid": "AllowCloudFrontServicePrincipalReadOnly",
        "Effect": "Allow",
        "Principal": {
            "Service": "cloudfront.amazonaws.com"
        },
        "Action": "s3:GetObject",
        "Resource": "arn:aws:s3:::cf-s3-ecs-demo-bucket/*",
        "Condition": {
            "StringEquals": {
                "AWS:SourceArn": "arn:aws:cloudfront::111122223333:distribution/EDFDVBD6EXAMPLE"
            }
        }
    }
}
```

You can find the CloudFront distribution ID by running `copilot env show --resources`.

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

- Download [the latest CLI version](../docs/getting-started/install.en.md)
- Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
- Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0)
