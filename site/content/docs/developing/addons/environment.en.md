# Modeling Additional Environment Resources with AWS CloudFormation

Additional AWS resources, referred to as "addons" in the CLI, are any additional AWS services that a [service or environment manifest](../../manifest/overview.en.md) does not integrate by default. 
For example, an addon can be a DynamoDB table, an S3 bucket, or an RDS Aurora Serverless cluster that your service needs to read or write to.

You can define additional resources for a workload (such as a [Load Balanced Web Service](../../manifest/lb-web-service.en.md)
or a [Scheduled Job](../../manifest/scheduled-job.en.md)).
The lifecycle of workload addons will be managed by the workload and will be deleted once the workload is deleted.

Alternatively, you can define additional shareable resource for an environment.
Environment addons won't be deleted unless the environment is deleted.

This page documents how to create environment-level addons.
For workload-level addons, visit [Modeling Additional Workload Resources with AWS CloudFormation](./workload.en.md).

## How do I add an S3 bucket, a DDB Table, or an Aurora Serverless cluster?

Copilot provides the following commands to help you create certain kinds of addons:

* [`storage init`](../../commands/storage-init.en.md) will create a DynamoDB table, an S3 bucket, or an Aurora Serverless cluster.

You can run `copilot storage init` from your workspace and be guided through some questions to help you set up these resources.


## How to add other resources?

For other types of addons, you can add your own custom CloudFormation templates:

1. You can store the custom templates in your workspace under `copilot/environments/addons` directory.
2. When running `copilot env deploy`, the custom addons template will be deployed along with your environment stack.


???- note "Sample workspace layout with environment addons"
    ```term
    .
    └── copilot
        └── environments
            ├── addons  # Store environment addons.
            │   └── mys3.yaml
            ├── dev
            └── prod      
    ```

## What does an addon template look like?
An environment addon template can be [any valid CloudFormation template](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-anatomy.html) that satisfies the following:

* Includes at least one [`Resource`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html).
* The `Parameters` section includes `App`, `Env`.

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
2. In your `addons.parameters.yml`, define the values of those additional parameters. They can refer to values from your environment stack. 

???- note "Examples: Customize addon parameters"
    ```yaml
    # In "environments/addons/my-addon.yml"
    Parameters:
      # Required parameters by AWS Copilot.
      App:
        Type: String
      Env:
        Type: String
      # Additional parameters defined in addons.parameters.yml
      ClusterName:
        Type: String
    ```
    ```yaml
    # In "environments/addons/addons.parameters.yml"
    Parameters:
        ClusterName: !Ref Cluster
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

#### Environment addon: Connecting to your workloads

A value from an environment addon can be referenced by a workload addon or a workload manifest.
To do so, you should first export the value from the environment addon using the `Outputs` section.

???+ note "Example: Export values from environment addons"
    ```yaml
    Outputs:
        MyTableARN:
            Value: !GetAtt ServiceTable.Arn
            Export:
                Name: !Sub ${App}-${Env}-MyTableARN  # This value can be consumed by a workload manifest or a workload addon.
        MyTableName:
            Value: !Ref ServiceTable
            Export:
                Name: !Sub ${App}-${Env}-MyTableName
    ```


It is important that you add the `Export` block. 
Otherwise, your workload stack or your workload addons won't be able to access the value.
You will use `Export.Name` to reference the value from your workload-level resources.

???- hint "Consideration: Namespace your `Export.Name`"
    You can specify any name you like for `Export.Name`.
    That is, it doesn't have to be prefixed with `!Sub ${App}-${Env}`; it can simply be `MyTableName`.

    However, within an AWS region, the `Export.Name` must be unique. 
    That is, you can't have two exports named `MyTableName` in `us-east-1`.
    
    Therefore, we recommend you to namespace your exports with `${App}` and `${Env}` to decrease the chance of name collision. 
    In addition, this makes it clear which application and environment the value is managed under.
    
    With the namespace, for example, say your application's name is `"my-app"`,
    and you deployed the addons with environment `test`, then the final export name would be rendered to `my-app-test-MyTableName`.


##### Referencing from a workload addon

In your workload addons, you can reference a value from your environment addons, as long as that value is exported.
To do so, use the [`Fn::ImportValue`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/intrinsic-function-reference-importvalue.html)
function with that value's export name to import it from an environment addon.

???- note "Example: An IAM policy to access an environment-level DynamoDB table"
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
        Description: Your workload's name.
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
                  - dynamodb:* # NOTE: Scope down the permissions in your real application. This is done so that this example isn't too long!
                Resource: 
                  Fn::ImportValue:                # <- We import the table ARN from the environment addons.
                    !Sub ${App}-${Env}-MyTableARN # <- The export name that we used.
    ```

##### Referencing from a workload manifest

You can also reference a value from your environment addons in a workload manifest for 
[`variables`](../../../manifest/lb-web-service/#variables-from-cfn), 
[`secrets`](../../../manifest/lb-web-service/#secrets-from-cfn) and 
[`security_groups`](../../../manifest/lb-web-service/#network-vpc-security-groups-from-cfn), 
as long as that value is exported. To do so, use `from_cfn` fields in workload manifests with that value's export name.


???- note "Examples: using `from_cfn`"
    === "Inject an environment variable"
        ```yaml
        name: db-front
        type: Backend Service
        variables:
          MY_TABLE_NAME:
            from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyTableName
        ```

    === "Inject a secret"
        ```yaml
        name: db-front
        type: Backend Service
        
        secrets:
          MY_CLUSTER_CREDS:
            from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyClusterSecret
        ```

    === "Attach a security group"
        ```yaml
        name: db-front
        type: Backend Service
        
        security_groups:
            - from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyClusterAllowedSecurityGroup
        ```



## Examples

### Environment Addons Walk-through
See our [v1.25.0 blog post](../../../../blogs/release-v125/#environment-addons) for a detailed walk-through!