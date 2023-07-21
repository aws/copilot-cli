# svc override
```console
$ copilot svc override
```

## What does it do?

Scaffold Infrastructure as Code (IaC) extension files for a service.
The generated files allow you to extend and override the Copilot-generated AWS CloudFormation template.
You can edit the files to change existing resource properties, and delete
or add new resources to the service's template.

### Learn more

To learn more, check out the guides for overriding with [YAML Patches](../developing/overrides/yamlpatch.md) and the
[AWS Cloud Development Kit](../developing/overrides/cdk.md).

## What are the flags?

```console
  -a, --app string            Name of the application.
      --cdk-language string   Optional. The Cloud Development Kit language. (default "typescript")
  -e, --env string            Optional. Name of the environment to use when retrieving resources in a template.
                              Defaults to a random environment.
  -h, --help                  Help for override
  -n, --name string           Name of the service.
      --skip-resources        Optional. Skip asking for which resources to override and generate empty IaC extension files.
      --tool string           Infrastructure as Code tool to override a template.
                              Must be one of: "cdk" or "yamlpatch".
```

## Example

Create a new Cloud Development Kit application to override the "frontend" service template.

```console
$ copilot svc override -n frontend --tool cdk
```

## What does it look like?

![svc-override](https://user-images.githubusercontent.com/879348/227581322-7ef52595-4d92-47ff-860a-329c29ae1e04.gif)
