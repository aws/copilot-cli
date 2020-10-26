# job package 
```bash
$ copilot job package
```

## What does it do?

`copilot job package` produces the CloudFormation template(s) used to deploy a job to an environment.

## What are the flags?

```bash
  -a, --app string          Name of the application.
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the job.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --tag string          Optional. The container image tag.
```

## Examples

Prints the CloudFormation template for the "report-generator" job parametrized for the "test" environment.
 
```bash
$ copilot job package -n report-generator -e test
```

Writes the CloudFormation stack and configuration to an "infrastructure/" sub-directory instead of printing.
  
```bash
$ copilot job package -n report-generator -e test --output-dir ./infrastructure
$ ls ./infrastructure
  report-generator-test.stack.yml      report-generator-test.params.yml
```