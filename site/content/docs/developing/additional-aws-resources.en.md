# Additional AWS Resources

Additional AWS resources, referred to as "addons" in the CLI, are any additional AWS services that a [service manifest](../manifest/overview.en.md) does not integrate by default. For example, an addon can be a DynamoDB table, an S3 bucket, or an RDS Aurora Serverless cluster that your service needs to read or write to.

## How do I add an S3 bucket, a DDB Table, or an Aurora Serverless cluster?

Copilot provides the following commands to help you create certain kinds of addons:

* [`storage init`](../commands/storage-init.en.md) will create a DynamoDB table, an S3 bucket, or an Aurora Serverless cluster.  

You can run `copilot storage init` from your workspace and be guided through some questions to help you set up these resources.

## How to do I add other resources?

For other types of addons, you can add your own custom CloudFormation templates according to the following instructions.

Let's say you have a service named `webhook` in your workspace:
```bash
.
└── copilot
    └── webhook
        └── manifest.yml
```
And you want to add a custom DynamoDB table to `webhook`. Then under the `webhook/` directory, create a new `addons/` directory and add a CloudFormation template for your instance.
```bash
.
└── copilot
    └── webhook
        ├── addons
        │   └── mytable-ddb.yaml
        └── manifest.yaml
```
Typically each file under the `addons/` directory represents a separate addon and is represented as an [AWS CloudFormation (CFN) template](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-anatomy.html). For example, if we want to also add an S3 bucket addon to our service then we could either run `storage init` or create our own custom, separate `mybucket-s3.yaml` file.
 
When your service gets deployed, Copilot merges all these files into a single AWS CloudFormation template and creates a nested stack under your service's stack.

## What does an addon template look like?
An addon template can be any valid CloudFormation template.   
However, by default, Copilot will pass the `App`, `Env`, and `Name` [Parameters](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/parameters-section-structure.html); you can customize your resource properties with [Conditions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/conditions-section-structure.html) or [Mappings](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/mappings-section-structure.html) if you wish to.

Here are several possible ways to access [Resources](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html) from your ECS task:

* If you need to add additional policies to your ECS task role, you can define an [IAM ManagedPolicy](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-iam-managedpolicy.html) resource in your template that holds the permissions for your task, and then add it as an [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html). The permission will be injected into your ECS Task Role.
* If you need to add a security group to your ECS service, you can define a [Security Group](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group.html) in your template, and then add it as an [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html). The security group will be automatically attached to your ECS service. 
* If you'd like to inject a secret, you can define a [Secret](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-secretsmanager-secret.html) in your template, and then add it as an [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html). The secret will be injected into your container and can be accessed as an environment variable as capital SNAKE_CASE. 
* If you'd like to inject any environment variable, you can create an [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html) for any value that you want to be injected as an environment variable to your ECS tasks. It will be injected into your container and accessed as an environment variable  as capital SNAKE_CASE.

Here is an example template layout for a DynamoDB table addon:
```yaml
# You can use any of these parameters to create conditions or mappings in your template.
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
  # Create your resource here, such as an AWS::DynamoDB::Table:
  # MyTable:
  #   Type: AWS::DynamoDB::Table
  #   Properties:
  #     ...

  # 1. In addition to your resource, if you need to access the resource from your ECS task 
  # then you need to create an AWS::IAM::ManagedPolicy that holds the permissions for your resource.
  #
  # For example, below is a sample policy for MyTable:
  MyTableAccessPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: 2012-10-17
        Statement:
          - Sid: DDBActions
            Effect: Allow
            Action:
              - dynamodb:BatchGet*
              - dynamodb:DescribeStream
              - dynamodb:DescribeTable
              - dynamodb:Get*
              - dynamodb:Query
              - dynamodb:Scan
              - dynamodb:BatchWrite*
              - dynamodb:Create*
              - dynamodb:Delete*
              - dynamodb:Update*
              - dynamodb:PutItem
            Resource: !Sub ${ MyTable.Arn}

Outputs:
  # 2. If you want to inject a property of your resource as an environment variable to your ECS task,
  # then you need to define an output for it.
  #
  # For example, the output MyTableName will be injected in capital snake case, MY_TABLE_NAME, to your task.
  MyTableName:
    Description: "The name of this DynamoDB."
    Value: !Ref MyTable

  # 1. You also need to output the IAM ManagedPolicy so that Copilot can inject it to your ECS task role.
  MyTableAccessPolicyArn:
    Description: "The ARN of the ManagedPolicy to attach to the task role."
    Value: !Ref MyTableAccessPolicy
```

On your next release, Copilot will include this template as a nested stack under your service!

!!! info
    We recommend following [Amazon IAM best practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html) while defining AWS Managed Policies for the additional resources, including:
    
    * [Grant least privilege](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#grant-least-privilege) to the policies defined in your `addons/` directory.  
    * [Use policy conditions for extra security](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#use-policy-conditions) to restrict your policies to access only the resources defined in your `addons/` directory.   

