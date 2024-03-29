# YAML Patch Overrides

{% include 'overrides-intro.md' %}

## When should I use YAML Patch over CDK overrides?

Both options are a "break the glass" mechanism to access and configure functionality that is not surfaced by Copilot [manifests](../../manifest/overview.md).

We recommend using YAML patch over the [AWS Cloud Development Kit (CDK) overrides](./cdk.md) if 1) you do not want to have a dependency
on any other tooling and framework (such as [Node.js](https://nodejs.org) and the [CDK](https://docs.aws.amazon.com/cdk/v2/guide/home.html)),
or 2) you have to write only a handful modifications.

## How to get started

You can extend your CloudFormation template with YAML patches by running the `copilot [noun] override` command.
For example, you can run `copilot svc override` to update the template of a Load Balanced Web Service.
The command will generate a sample `cfn.patches.yml` file under the `copilot/[name]/overrides` directory.

## How does it work?

The syntax of `cfn.patches.yml` conforms to [RFC6902: JSON Patch](https://www.rfc-editor.org/rfc/rfc6902). Currently,
the CLI supports three operations: `add`, `remove`, and `replace`. Here is a sample `cfn.patches.yml` file:

```yaml
- op: add
  path: /Mappings
  value:
    ContainerSettings:
      test: { Cpu: 256, Mem: 512 }
      prod: { Cpu: 1024, Mem: 1024}
- op: remove
  path: /Resources/TaskRole
- op: replace
  path: /Resources/TaskDefinition/Properties/ContainerDefinitions/1/Essential
  value: false
- op: add
  path: /Resources/Service/Properties/ServiceConnectConfiguration/Services/0/ClientAliases/-
  value:
    Port: !Ref TargetPort
    DnsName: yamlpatchiscool
```

Each patch is applied sequentially to the CloudFormation template. The resulting template becomes the target of the next patch.
Evaluation continues until all patches are successfully applied or an error is encountered.

### Path evaluation

The `path` field of a patch conforms to the [RFC6901: JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901) syntax.

- Each `path` value is separated by the `/` character and evaluation stops once the target CloudFormation property is reached.
- If the target path is an array, the reference token must be either:
    - characters comprised of digits starting at 0.
    - exactly the single character `-` when the operation is `add`, to append to the array.

## Additional Examples

To add a new property to an existing resource:

```yaml
- op: add
  path: /Resources/LogGroup/Properties/Tags
  value:
    - Key: keyname
      Value: value1
```

To add a new property in a specific index of an array:

```yaml
- op: add
  path: /Resources/TaskDefinition/Properties/ContainerDefinitions/0/EnvironmentFiles/0
  value: arn:aws:s3:::bucket_name/key_name
```

To add a new element at the end of an array:

```yaml
- op: add
  path: /Resources/TaskRole/Properties/Policies/-
  value:
    PolicyName: DynamoDBReader
    PolicyDocument:
      Version: "2012-10-17"
      Statement:
        - Effect: Allow
          Action:
            - dynamodb:Get*
          Resource: '*'
```

To replace an existing property's value:

```yaml
- op: replace
  path: /Resources/LogGroup/Properties/RetentionInDays
  value: 60
```

To delete an element from an array, you must reference the exact index:

```yaml
- op: remove
  path: /Resources/ExecutionRole/Properties/Policies/0/PolicyDocument/Statement/1/Action/0
```

To delete an entire resource:

```yaml
- op: remove
  path: /Resources/ExecutionRole
```
