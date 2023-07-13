# copilot env override
```console
$ copilot env override
```

## What does it do?
Scaffold Infrastructure as Code (IaC) extension files for environments.
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
  -n, --name string           Optional. Name of the environment to use when retrieving resources in a template.
                              Defaults to a random environment.
      --skip-resources        Optional. Skip asking for which resources to override and generate empty IaC extension files.
      --tool string           Infrastructure as Code tool to override a template.
                              Must be one of: "cdk" or "yamlpatch".
```

## Example

Create a new Cloud Development Kit application to override environment templates.

```console
$ copilot env override --tool cdk
```

## What does it look like?

![env-override](https://user-images.githubusercontent.com/879348/227585768-44d5d91f-11d5-4d4b-a5fa-12bb5239710f.gif)