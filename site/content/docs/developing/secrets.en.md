# Secrets

Secrets are sensitive bits of information like OAuth tokens, secret keys or API keys - information that you need in your application code, but shouldn't commit to your source code. In the AWS Copilot CLI, secrets are passed in as environment variables (read more about [developing with environment variables](../developing/environment-variables.en.md)), but they're treated differently due to their sensitive nature. 

## How do I add Secrets?

Adding secrets currently requires you to store your secret as a secure string in [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) (SSM), then add a reference to the SSM parameter to your [manifest](../manifest/overview.en.md). 

You can easily create secrets using [`copilot secret init`](https://aws.github.io/copilot-cli/docs/commands/secret-init/)! After creating the secrets, Copilot will tell you what your secrets' names are. You can then use the name to add the reference in your manifest. 

### Alternatively...

If you want to bring your own secrets, be sure to add two tags to your secrets - `copilot-application: <application from which you want to access the secret>` and 
`copilot-environment: <environment from which you want to access the secret>.`

Copilot requires the `copilot-application` and `copilot-environment` tags to limit access to this secret.  

Suppose you have a (properly tagged!) SSM parameter named `GH_WEBHOOK_SECRET` with value `secretvalue1234`, you can modify your manifest file to pass in this value:

```yaml
secrets:                      
  GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET  
```

Once you deploy this update in manifest, your service or job will be able to access the environment variable `GITHUB_WEBHOOK_SECRET`, which will have the value of the SSM parameter `GH_WEBHOOK_SECRET`, `secretvalue1234`.

This works because ECS Agent will resolve the SSM parameter when it starts up your task, and set the environment variable for you.

!!! attention
    Secrets are not supported for Request-Driven Web Services.

