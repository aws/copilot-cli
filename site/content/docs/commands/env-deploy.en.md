# env deploy <span class="version" > added in [v1.20.0](../../blogs/release-v120.en.md) </span>
```console
$ copilot env deploy
```

## What does it do?

`copilot env deploy` takes the configurations in your [environment manifest](../manifest/environment.en.md) and deploys your environment infrastructure.

## What are the flags?

```
  -a, --app string    Name of the application.
      --diff          Compares the generated CloudFormation template to the deployed stack.
      --force         Optional. Force update the environment stack template.
  -h, --help          help for deploy
  -n, --name string   Name of the environment.
      --no-rollback   Optional. Disable automatic stack
                      rollback in case of deployment failure.
                      We do not recommend using this flag for a
                      production environment.
```
