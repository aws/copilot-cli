# env package
```console
$ copilot env package [flags]
```

## What does it do?
`copilot env package` prints the CloudFormation stack template and configuration used to deploy an environment.

## What are the flags?
```console
      --allow-downgrade     Optional. Allow using an older version of Copilot to update Copilot components
                            updated by a newer version of Copilot.
  -a, --app string          Name of the application.
      --diff                Compares the generated CloudFormation template to the deployed stack.
      --force               Optional. Force update the environment stack template.
  -h, --help                help for package
  -n, --name string         Name of the environment.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --upload-assets       Optional. Whether to upload assets (container images, Lambda functions, etc.).
                            Uploaded asset locations are filled in the template configuration.
```

## Examples
Print the CloudFormation template for the "prod" environment and upload custom resources.
```console
$ copilot env package -n prod --upload-assets
```
Write the CloudFormation template and configuration to a "infrastructure/" sub-directory instead of stdout.
```console
$ copilot env package -n test --output-dir ./infrastructure --upload-assets
$ ls ./infrastructure
test.env.yml      test.env.params.json
```

Use `--diff` to print the diff and exit.
```console
$ copilot env deploy --diff
~ Resources:
    ~ Cluster:
        ~ Properties:
            ~ ClusterSettings:
                ~ - (changed item)
                  ~ Value: enabled -> disabled
```

!!! info "The exit codes when using `copilot [noun] package --diff`"
    0 = no diffs found  
    1 = diffs found  
    2 = error producing diffs
