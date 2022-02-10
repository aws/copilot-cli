# svc package
```bash
$ copilot svc package
```

## What does it do?

`copilot svc package` produces the CloudFormation template(s) used to deploy a service to an environment.

## What are the flags?

```bash
  -a, --app string          Name of the application.
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the service.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --tag string          Optional. The service's image tag.
      --upload-resources    Optional. Whether to upload resources that are required for deployment.
```

## Example

Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.

```bash
$ copilot svc package -n frontend -e test --output-dir ./infrastructure
$ ls ./infrastructure
frontend.stack.yml      frontend-test.config.yml
```
