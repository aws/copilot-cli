# secret init
```console
$ copilot secret init
```

## What does it do?
`copilot secret init` creates or updates secrets as [SecureString parameters](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html#what-is-a-parameter) in SSM Parameter Store for your application.

A secret can have different values in each of your existing environments, and is accessible by your services or jobs from the same application and environment.

## What are the flags?
```
  -a, --app string              Name of the application.
      --cli-input-yaml string   Optional. A YAML file in which the secret values are specified.
                                Mutually exclusive with the -n, --name and --values flags.
  -h, --help                    help for init
  -n, --name string             The name of the secret.
                                Mutually exclusive with the --cli-input-yaml flag.
      --overwrite               Optional. Whether to overwrite an existing secret.
      --values stringToString   Values of the secret in each environment. Specified as <environment>=<value> separated by commas.
                                Mutually exclusive with the --cli-input-yaml flag. (default [])
```
## How can I use it?
Create a secret with prompts. You will be prompted for the name of the secret, and its values in each of your existing environments.
```console
$ copilot secret init
```

Create a secret named `db_password` in multiple environments. You will be prompted for the `db_password`'s values you want for each of your existing environments.
```console
$ copilot secret init --name db_password
```
Create secrets from `input.yml`. For the format of the YAML file, please see <a href="#secret-init-cli-input-yaml">below</a>.
```console
$ copilot secret init --cli-input-yaml input.yml
```

!!!info
    It is recommended that you specify your secret's values through our prompts (e.g. by running `copilot secret init --name`) or from an input file by using the `--cli-input-yaml` flag. While the `--values` flag is a convenient way to specify secret values, your input may appear in your shell history as plaintext.

## What's next?

Copilot will create SSM parameters named `/copilot/<app name>/<env name>/secrets/<secret name>`. 
Using the parameter names, you can then modify the `secrets` section in your [service's](../manifest/backend-service.en.md#secrets) or [job's](../manifest/scheduled-job.en.md#secrets) manifest to reference the secrets that were created. 

For example, suppose you have an application `my-app`, and you've created a secret `db_host` in your `prod` and `dev` environments.
You can modify your service's manifest as follows:
```yaml
environments:
    prod:
      secrets: 
        DB_PASSWORD: /copilot/my-app/prod/secrets/db_password
    dev:
      secrets:
        DB_PASSWORD: /copilot/my-app/dev/secrets/db_password
```

Once you deploy this updated manifest, your service or job will be able to access the environment variable `DB_PASSWORD`.
It will have the value of the SSM parameter `/copilot/my-app/prod/secrets/db_password` if the service/job is deployed in a `prod` environment, and `/copilot/my-app/dev/secrets/db_password` if it's deployed in a `dev` environment.

This works because ECS Agent will resolve the SSM parameter when it starts up your task, and set the environment variable for you.

## <span id="secret-init-cli-input-yaml">How do I use the `--cli-input-yaml` flag?</span>
You can specify multiple secrets and their values in each of your existing environments in a file. Then you can use the file as the input to `--cli-input-yaml` flag. Copilot will read from the file and create or update the secrets accordingly.

The YAML file should be formatted as follows:
```yaml
<secret A>:
  <env 1>: <the value of secret A in env 1>
  <env 2>: <the value of secret A in env 2>
  <env 3>: <the value of secret A in env 3>
<secret B>:
  <env 1>: <the value of secret B in env 1>
  <env 2>: <the value of secret B in env 2>
```

Here is an example input file that creates secrets `db_host` and `db_password` in `dev`, `test` and `prod`, and `notification_email` in `dev` and `test` environments. Note that `notification_email` won't be created for the `prod` environment since it doesn't have a value for `prod`.
```yaml
db_host:
  dev: dev.db.host.com
  test: test.db.host.com
  prod: prod.db.host.com
db_password:
  dev: dev-db-pwd
  test: test-db-pwd
  prod: prod-db-pwd
notification_email:
  dev: dev@email.com
  test: test@email.com
```
