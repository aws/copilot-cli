# Secrets

Secrets are sensitive bits of information like OAuth tokens, secret keys or API keys - information that you need in your application code, 
but shouldn't commit to your source code. In the AWS Copilot CLI, secrets are passed in as environment variables 
(read more about [developing with environment variables](../developing/environment-variables.en.md)), but they're treated differently due to their sensitive nature.

## How do I add Secrets?

Adding secrets requires you to store your secret in [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) (SSM)
or in [AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/latest/userguide/intro.html), then add a reference to the secret in your [manifest](../manifest/overview.en.md).

You can easily create a secret in SSM as a `SecureString` using [`copilot secret init`](../commands/secret-init.en.md)! 

## Bring Your Own Secrets

### In SSM
If you want to bring your own secrets, be sure to add two tags to your secrets:  

| Key                     | Value                                                       |
| ----------------------- | ----------------------------------------------------------- |
| `copilot-application`   | Application name from which you want to access the secret   |
| `copilot-environment`   | Environment name from which you want to access the secret   |

Copilot requires the `copilot-application` and `copilot-environment` tags to limit access to this secret.  

Suppose you have a (properly tagged!) SSM parameter named `GH_WEBHOOK_SECRET` with value `secretvalue1234`. You can modify your manifest file to pass in this value:

```yaml
secrets:                      
  GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET  
```

Once you deploy this updated manifest, your service or job will be able to access the environment variable `GITHUB_WEBHOOK_SECRET`, which will have the value of the SSM parameter `GH_WEBHOOK_SECRET`, `secretvalue1234`.  
This works because ECS Agent will resolve the SSM parameter when it starts up your task, and set the environment variable for you.

### In Secrets Manager
Similar to SSM, first ensure that your Secrets Manager secret has the `copilot-application` and `copilot-environment` tags.  

Suppose you have a Secrets Manager secret with the following configuration:

| Field  | Value                                                                                                                                                                 |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Name   | `demo/test/mysql`                                                                                                                                                     |
| ARN    | `arn:aws:secretsmanager:us-west-2:111122223333:secret:demo/test/mysql-Yi6mvL`                                                                                        |
| Value  | `{"engine": "mysql","username": "user1","password": "i29wwX!%9wFV","host": "my-database-endpoint.us-east-1.rds.amazonaws.com","dbname": "myDatabase","port": "3306"`} |
| Tags   | `copilot-application=demo`, `copilot-environment=test` |


You can modify your manifest file with:
```yaml
secrets:
  # (Recommended) Option 1. Referring to the secret by name.
  DB:
    secretsmanager: 'demo/test/mysql'
  # You can refer to a specific key in the JSON blob.
  DB_PASSWORD:
    secretsmanager: 'demo/test/mysql:password::'
  # You can substitute predefined environment variables to keep your manifest succinct.
  DB_PASSWORD:
    secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql:password::'

  # Option 2. Alternatively, you can refer to the secret by ARN.
  DB: "'arn:aws:secretsmanager:us-west-2:111122223333:secret:demo/test/mysql-Yi6mvL'"
```