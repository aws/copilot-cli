# job package
```console
$ copilot job package
```

## What does it do?

`copilot job package` produces the CloudFormation template(s) used to deploy a job to an environment.

## What are the flags?

```
      --allow-downgrade     Optional. Allow using an older version of Copilot to update Copilot components
                            updated by a newer version of Copilot.
  -a, --app string          Name of the application.
      --diff                Compares the generated CloudFormation template to the deployed stack.
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the job.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --tag string          Optional. The tag for the container images Copilot builds from Dockerfiles.
      --upload-assets       Optional. Whether to upload assets (container images, Lambda functions, etc.).
                            Uploaded asset locations are filled in the template configuration.
```

## Examples

Prints the CloudFormation template for the "report-generator" job parametrized for the "test" environment.

```console
$ copilot job package -n report-generator -e test
```

Writes the CloudFormation stack and configuration to an "infrastructure/" sub-directory instead of printing.

```console
$ copilot job package -n report-generator -e test --output-dir ./infrastructure
$ ls ./infrastructure
  report-generator-test.stack.yml      report-generator-test.params.yml
```

Use `--diff` to print the diff and exit.
```console
$ copilot job deploy --diff
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
