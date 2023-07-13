# copilot pipeline override
```console
$ copilot pipeline override
```

## What does it do?
Scaffold Infrastructure as Code (IaC) extension files for a pipeline.
The generated files allow you to extend and override the Copilot-generated AWS CloudFormation template.
You can edit the files to change existing resource properties, and delete
or add new resources to an environment's template.

### Learn more

To learn more, check out the guides for overriding with [YAML Patches](../developing/overrides/yamlpatch.md) and the
[AWS Cloud Development Kit](../developing/overrides/cdk.md).

## What are the flags?

```console
  -a, --app string            Name of the application.
      --cdk-language string   Optional. The Cloud Development Kit language. (default "typescript")
  -h, --help                  Help for override
  -n, --name string           Name of the pipeline.
      --skip-resources        Optional. Skip asking for which resources to override and generate empty IaC extension files.
      --tool string           Infrastructure as Code tool to override a template.
                              Must be one of: "cdk" or "yamlpatch".
```

## Example

Create a new Cloud Development Kit application to override the "myrepo-main" pipeline template.

```console
$ copilot pipeline override -n myrepo-main --tool cdk
```

## What does it look like?

![pipeline-override](https://github.com/aws/copilot-cli/assets/10566468/21ecf58b-fc7e-4e20-a5b7-6b8e2049fda4)