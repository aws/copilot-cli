# env deploy <span class="version" > added in [v1.20.0](../../blogs/release-v120.en.md) </span>
```console
$ copilot env deploy
```

## What does it do?

`copilot env deploy` takes the configurations in your [environment manifest](../manifest/environment.en.md) and deploys your environment infrastructure.

## What are the flags?

```
      --allow-downgrade   Optional. Allow using an older version of Copilot to update Copilot components
                          updated by a newer version of Copilot.
  -a, --app string        Name of the application.
      --detach            Optional. Skip displaying CloudFormation deployment progress.
      --diff              Compares the generated CloudFormation template to the deployed stack.
      --diff-yes          Skip interactive approval of diff before deploying.
      --force             Optional. Force update the environment stack template.
  -h, --help              help for deploy
  -n, --name string       Name of the environment.
      --no-rollback       Optional. Disable automatic stack
                          rollback in case of deployment failure.
                          We do not recommend using this flag for a
                          production environment.
```

## Examples
Use `--diff` to see what will be changed before making a deployment.

```console
$ copilot env deploy --name test --diff
~ Resources:
    ~ Cluster:
        ~ Properties:
            ~ ClusterSettings:
                ~ - (changed item)
                  ~ Value: enabled -> disabled

Continue with the deployment? (y/N)
```

!!!info "`copilot env package --diff`"
    Alternatively, if you just wish to take a peek at the diff without potentially making a deployment,
    you can run `copilot env package --diff`, which will print the diff and exit.
