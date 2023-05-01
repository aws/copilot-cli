# Modeling Additional Workload Resources with AWS CloudFormation

Additional AWS resources, referred to as "addons" in the CLI, are any additional AWS services that a [service or environment manifest](../../manifest/overview.en.md) does not integrate by default. 
For example, an addon can be a DynamoDB table, an S3 bucket, or an RDS Aurora Serverless cluster that your service needs to read or write to.

You can define additional resources for a workload (such as a [Load Balanced Web Service](../../manifest/lb-web-service.en.md) 
or a [Scheduled Job](../../manifest/scheduled-job.en.md)). 
The lifecycle of workload addons will be managed by the workload and will be deleted once the workload is deleted.  

Alternatively, you can define additional shareable resource for an environment. 
Environment addons won't be deleted unless the environment is deleted.

This page documents how to create workload-level addons. 
For environment-level addons, visit [Modeling Additional Environment Resources with AWS CloudFormation](../environment).

## How do I add an S3 bucket, a DDB Table, or an Aurora Serverless cluster?

Copilot provides the following commands to help you create certain kinds of addons:

* [`storage init`](../../commands/storage-init.en.md) will create a DynamoDB table, an S3 bucket, or an Aurora Serverless cluster.  

You can run `copilot storage init` from your workspace and be guided through some questions to help you set up these resources.


## How to add other resources?

For other types of addons, you can add your own custom CloudFormation templates:

1. You can store the custom templates in your workspace under `copilot/<workload>/addons` directory.
2. When running `copilot [svc/job] deploy`, the custom addons template will be deployed along with your workload stack.


???- note "Sample workspace layout with workload addons"
    ```term
    .
    └── copilot
        └── webhook
            ├── addons # Store addons associated with the service "webhook".
            │   └── mytable-ddb.yaml
            └── manifest.yaml 
    ```

## What does an addon template look like?
A workload addon template can be [any valid CloudFormation template](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-anatomy.html) that satisfies the following:

* Include at least one [`Resource`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html).
* The [`Parameters`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/parameters-section-structure.html) section must include `App`, `Env`, `Name`.

you can customize your resource properties with [Conditions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/conditions-section-structure.html) or [Mappings](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/mappings-section-structure.html).

!!! info ""
    We recommend following [Amazon IAM best practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html) while defining AWS Managed Policies for the additional resources, including:

    * [Grant least privilege](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#grant-least-privilege) to the policies defined in your `addons/` directory.  
    * [Use policy conditions for extra security](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#use-policy-conditions) to restrict your policies to access only the resources defined in your `addons/` directory.   


### Writing the `Parameters` section

There are a few parameters that Copilot requires you to define in your templates. 

!!! info ""
    ```yaml
    Parameters:
        App:
            Type: String
        Env:
            Type: String
        Name:
            Type: String
    ```


#### Customizing the `Parameters` section

If you'd like to define parameters in addition to the ones required by Copilot, you can do so with a
`addons.parameters.yml` file.

```term
.
└── addons/
    ├── template.yml
    └── addons.parameters.yml # Add this file under your addons/ directory.
```

1. In your template file, add the additional parameters under the `Parameters` section.
2. In your `addons.parameters.yml`, define the values of those additional parameters. They can refer to values from your workload stack. 

???- note "Examples: Customize addon parameters"
    ```yaml
    # In "webhook/addons/my-addon.yml"
    Parameters:
      # Required parameters by AWS Copilot.
      App:
        Type: String
      Env:
        Type: String
      Name:
        Type: String
      # Additional parameters defined in addons.parameters.yml
      ServiceName:
        Type: String
    ```
    ```yaml
    # In "webhook/addons/addons.parameters.yml"
    Parameters:
        ServiceName: !GetAtt Service.Name
    ```

### Writing the `Conditions` and the `Mappings` sections

Often, you want to configure your addon resources differently depending on certain conditions. 
For example, you could conditionally configure your DB resource's capacity depending on whether it is deploying to a 
production or a test environment. To do so, you can use the `Conditions` section and the `Mappings` section.

???- note "Examples: Configure addons conditionally"
    === "Using `Mappings`"
        ```yaml
        Mappings:
            MyAuroraServerlessEnvScalingConfigurationMap:
                dev:
                    "DBMinCapacity": 0.5
                    "DBMaxCapacity": 8   
                test:
                    "DBMinCapacity": 1
                    "DBMaxCapacity": 32
                prod:
                    "DBMinCapacity": 1
                    "DBMaxCapacity": 64
        Resources:
            MyCluster:
                Type: AWS::RDS::DBCluster
                Properties:
                    ScalingConfiguration:
                        MinCapacity: !FindInMap
                            - MyAuroraServerlessEnvScalingConfigurationMap
                            - !Ref Env
                            - DBMinCapacity
                        MaxCapacity: !FindInMap
                            - MyAuroraServerlessEnvScalingConfigurationMap
                            - !Ref Env
                            - DBMaxCapacity
        ```
    
    === "Using `Conditions`"
        ```yaml
        Conditions:
          IsProd: !Equals [!Ref Env, "prod"] 
        
        Resources:
          MyCluster:
            Type: AWS::RDS::DBCluster
            Properties:
              ScalingConfiguration:
                  MinCapacity: !If [IsProd, 1, 0.5]
                  MaxCapacity: !If [IsProd, 8, 64]
        ```


### Writing the [`Outputs`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html) section

You can use the `Outputs` section to define any values that can be consumed by other resources; for example, a service,
a CloudFormation stack, etc.

#### Workload addons: Connecting to your workloads

Here are several possible ways to access addon [`Resources`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html) 
from your ECS task or App Runner instance:

* If you need to add additional policies to your ECS task role or App Runner instance role, you can define an [IAM ManagedPolicy](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-iam-managedpolicy.html) addon resource in your template that holds the additional permissions, and then [output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html) it. The permission will be injected into your task or instance role.
* If you need to add a security group to your ECS service, you can define a [Security Group](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group.html) in your template, and then add it as an [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html). The security group will be automatically attached to your ECS service.
* If you'd like to inject a secret to your ECS task, you can define a [Secret](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-secretsmanager-secret.html) in your template, and then add it as an [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html). The secret will be injected into your container and can be accessed as an environment variable in capital SNAKE_CASE.
* If you'd like to inject any resource value as an environment variable, you can create an [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html) to your ECS tasks. It will be injected into your container and may be accessed as an environment variable in capital SNAKE_CASE.

## Examples

### A Workload Addon Template For A DynamoDB Table

Here is an example template for a workload-level DynamoDB table addon:
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
    Description: Your workload's name.

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
        Version: '2012-10-17'
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
  # 1. You need to output the IAM ManagedPolicy so that Copilot can add it as a managed policy to your ECS task role.
  MyTableAccessPolicyArn:
    Description: "The ARN of the ManagedPolicy to attach to the task role."
    Value: !Ref MyTableAccessPolicy

  # 2. If you want to inject a property of your resource as an environment variable to your ECS task,
  # then you need to define an output for it.
  #
  # For example, the output MyTableName will be injected in capital snake case, MY_TABLE_NAME, to your task.
  MyTableName:
    Description: "The name of this DynamoDB."
    Value: !Ref MyTable
```

