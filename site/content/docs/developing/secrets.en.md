# Secrets

Secrets are sensitive bits of information like OAuth tokens, secret keys or API keys - information that you need in your application code, 
but shouldn't commit to your source code. In the AWS Copilot CLI, secrets are passed in as environment variables 
(read more about [developing with environment variables](../developing/environment-variables.en.md)), but they're treated differently due to their sensitive nature.

## How do I add Secrets?

Adding secrets requires you to store your secret as a SecureString in [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) (SSM)
or in [AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/latest/userguide/intro.html), then add a reference to the secret in your [manifest](../manifest/overview.en.md). 

You can easily create secrets in SSM using [`copilot secret init`](../commands/secret-init.en.md)! 

!!! attention
    Secrets are not supported for Request-Driven Web Services.

### Bring Your Own Secrets

#### In SSM
If you want to bring your own secrets, be sure to add two tags to your secrets:  

| Key                     | Value                                                       |
| ----------------------- | ----------------------------------------------------------- |
| `copilot-application`   |  Application name from which you want to access the secret  |
| `copilot-environment`   | Environment name from which you want to access the secret   |

Copilot requires the `copilot-application` and `copilot-environment` tags to limit access to this secret.  

Suppose you have a (properly tagged!) SSM parameter named `GH_WEBHOOK_SECRET` with value `secretvalue1234`. You can modify your manifest file to pass in this value:

```yaml
secrets:                      
  GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET  
```

Once you deploy this updated manifest, your service or job will be able to access the environment variable `GITHUB_WEBHOOK_SECRET`, which will have the value of the SSM parameter `GH_WEBHOOK_SECRET`, `secretvalue1234`.  
This works because ECS Agent will resolve the SSM parameter when it starts up your task, and set the environment variable for you.

#### In Secrets Manager
Similar to SSM, first ensure that your Secrets Manager secret has the `copilot-application` and `copilot-environments` tags.  

Now if you want to pass the Secrets Manager secret named `GH_WEBHOOK_SECRET` with ARN `arn:aws:secretsmanager:us-west-2:111122223333:secret:GH_WEBHOOK_SECRET-WC3PHL`, you can modify your manifest file with:
```yaml
secrets:
  # (Recommended) Option 1. Referring to the secret by name.
  GITHUB_WEBHOOK_SECRET:
    secretsmanager: GH_WEBHOOK_SECRET

  # Option 2. Referring to the secret by ARN.
  GITHUB_WEBHOOK_SECRET: '"arn:aws:secretsmanager:us-west-2:111122223333:secret:GH_WEBHOOK_SECRET-WC3PHL"'
```