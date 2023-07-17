# svc package
```console
$ copilot svc package
```

## What does it do?

`copilot svc package` produces the CloudFormation template(s) used to deploy a service to an environment.

## What are the flags?

```
      --allow-downgrade     Optional. Allow using an older version of Copilot to update Copilot components
                            updated by a newer version of Copilot.
  -a, --app string          Name of the application.
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the service.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --tag string          Optional. The service's image tag.
      --upload-assets       Optional. Whether to upload assets (container images, Lambda functions, etc.).
                            Uploaded asset locations are filled in the template configuration.
```

## Example

Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.

```console
$ copilot svc package -n frontend -e test --output-dir ./infrastructure
$ ls ./infrastructure
frontend.stack.yml      frontend-test.config.yml
```


Use `--diff` to print the diff and exit.
```console
$ copilot svc deploy --diff
~ Resources:
    ~ TaskDefinition:
        ~ Properties:
            ~ ContainerDefinitions:
                ~ - (changed item)
                  ~ Environment:
                      (4 unchanged items)
                      + - Name: LOG_LEVEL
                      +   Value: "info"
```

!!! info "The exit codes when using `copilot [noun] package --diff`"
    0 = no diffs found  
    1 = diffs found  
    2 = error producing diffs
