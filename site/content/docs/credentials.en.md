AWS Copilot uses AWS credentials to access the AWS API, store and look up an [application's metadata](concepts/applications.en.md), and deploy and operate an application's workloads.

You can learn more on how to configure AWS credentials in the [AWS CLI's documentation](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html).

## Application credentials
Copilot uses the AWS credentials from the [default credential provider chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials) to store and look up your [application's metadata](concepts/applications.en.md): which services and environments belong to it. 

!!! tip
    We **recommend using a [named profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html)** to store your application's credentials. 

The most convenient way is having the `[default]` profile point to your application's credentials:
```ini
# ~/.aws/credentials
[default]
aws_access_key_id=AKIAIOSFODNN7EXAMPLE
aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# ~/.aws/config
[default]
region=us-west-2
```
Alternatively, you can set the `AWS_PROFILE` environment variable to point to a different named profile. For example, we can have a `[my-app]` profile that can be used for your Copilot application instead of the `[default]` profile.

!!! note
    You **cannot** use the AWS account root user credentials for your application. Please first create an IAM user instead as described [here](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_root-user.html).

```ini
# ~/.aws/config
[profile my-app]
credential_process = /opt/bin/awscreds-custom --username helen
region=us-west-2

# Then you can run your Copilot commands leveraging the alternative profile:
$ export AWS_PROFILE=my-app
$ copilot deploy
```

!!! caution
    We **do not** recommend using the environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` directly to look up your application's metadata because if they're overridden or expired, Copilot will not be able to look up your services or environments. 

To learn more about all the supported `config` file settings: [Configuration and credential file settings](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html#cli-configure-files-settings).

## Environment credentials
Copilot [environments](concepts/environments.en.md) can be created in AWS accounts and regions separate from your application's. While initializing an environment, Copilot will prompt you to enter temporary credentials or a [named profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) to create your environment:
```bash
$ copilot env init

Name: prod-iad

  Which credentials would you like to use to create prod-iad?
  > Enter temporary credentials
  > [profile default]
  > [profile test]
  > [profile prod-iad]
  > [profile prod-pdx]
```
Unlike the [Application credentials](#application-credentials), the AWS credentials for an environment are only needed for creation or deletion. Therefore, it's safe to use the values from temporary environment variables. Copilot prompts or takes the credentials as flags because the default chain is reserved for your application credentials.
