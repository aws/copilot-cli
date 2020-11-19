# Secrets

Secrets are sensitive bits of information like OAuth tokens, secret keys or API keys - information that you need in your application code, but shouldn't commit to your source code. In the AWS Copilot CLI, secrets are passed in as environment variables (read more about [developing with environment variables](../developing/environment-variables.md)), but they're treated differently due to their sensitive nature. 

## How do I add Secrets?

Adding secrets currently requires you to store your secret as a secure string in [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) (SSM), then add a reference to the SSM parameter to your [manifest](../manifest/overview.md). 

We'll walk through an example where we want to store a secret called `GH_WEBHOOK_SECRET` with the value `secretvalue1234`. 

First, store the secret in SSM like so:

```sh
aws ssm put-parameter --name GH_WEBHOOK_SECRET --value secretvalue1234 --type SecureString\
  --tags Key=copilot-environment,Value=${ENVIRONMENT_NAME} Key=copilot-application,Value=${APP_NAME}
```

This will store the value `secretvalue1234` into the SSM parameter `GH_WEBHOOK_SECRET` as a secret. 

!!! attention
    Copilot requires the `copilot-application` and `copilot-environment` tags to limit access to this secret.  
    It's important to replace the `${ENVIRONMENT_NAME}` and `${APP_NAME}` with the Copilot application and environment you want to have access to this secret.


Next, we'll modify our manifest file to pass in this value:

```yaml
secrets:                      
  GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET  
```

Once we deploy this update to our manifest, we'll be able to access the environment variable `GITHUB_WEBHOOK_SECRET` which will have the value of the SSM parameter `GH_WEBHOOK_SECRET`, `secretvalue1234`.

This works because ECS Agent will resolve the SSM parameter when it starts up your task, and set the environment variable for you. 

!!! info
    **We're going to make this easier!** There are a couple of caveats - you have to store the secret in the same environment as your application.  
    Some of our next work is to add a `secrets` command that lets you add a secret without having to worry about which environment you're in, or how SSM works.
