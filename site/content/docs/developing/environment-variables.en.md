# Environment Variables

Environment variables are variables that are available to your service, based on the environment they're running in. Your service can reference them without having to define them. Environment variables are useful for when you want to pass in data to your service that's specific to a particular environment. For example, your test database name versus your production database name.

Accessing environment variables is usually simply based on the language you're using. Here are some examples of getting an environment variable called `DATABASE_NAME` in a few different languages.

__Go__
```go
dbName := os.Getenv("DATABASE_NAME")
```

__Javascript__
```javascript
var dbName = process.env.DATABASE_NAME;
```

__Python__
```python
database_name = os.getenv('DATABASE_NAME')
```

## What are the Default Environment Variables?

By default, the AWS Copilot CLI passes in some default environment variables for your service to use.

* `COPILOT_APPLICATION_NAME` - this is the name of the application this service is running in.
* `COPILOT_ENVIRONMENT_NAME` - this is the name of the environment the service is running in (test vs prod, for example)
* `COPILOT_SERVICE_NAME` - this is the name of the current service.
* `COPILOT_LB_DNS` - this is the DNS name of the Load Balancer (if it exists) such as _kudos-Publi-MC2WNHAIOAVS-588300247.us-west-2.elb.amazonaws.com_. Note: if you're using a custom domain name, this value will still be the Load Balancer's DNS name.
* `COPILOT_SERVICE_DISCOVERY_ENDPOINT` - this is the endpoint to add after a service name to talk to another service in your environment via service discovery. The value is `{env name}.{app name}.local`. For more information about service discovery, check out our [Service Discovery guide](../developing/svc-to-svc-communication.en.md#service-discovery).

## How do I add my own Environment Variables?

Adding your own environment variable is easy. You can add them directly to your [manifest](../manifest/overview.en.md) in the `variables` section. The following snippet will pass a environment variable called `LOG_LEVEL` to your service, with the value set to `debug`.

```yaml
# in copilot/{service name}/manifest.yml
variables:                    
  LOG_LEVEL: debug
```

You can also pass in a specific value for an environment variable based on the environment. We'll follow the same example as above, by setting the log level, but overriding the value to be `info` in our production environment. Changes to your manifest take effect when you deploy them, so changing them locally is safe.

```yaml
# in copilot/{service name}/manifest.yml
variables:                    
  LOG_LEVEL: debug

environments:
  production:
    variables:
      LOG_LEVEL: info
```

Here's a quick guide showing you how to add environment variables to your app by editing the manifest 👇

![Editing the manifest to add env vars](https://raw.githubusercontent.com/kohidave/ecs-cliv2-demos/master/env-vars-edit.svg?sanitize=true)

Additionally, if you want to add environment variables in bulk, you can list them in an [env file](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations). And then specify its path (from the root of the workspace) in the `env_file` field of your [manifest](../manifest/overview.en.md).

You may specify an env file at the root of your workspace for the main container, in any sidecar container definition, or under the `logging` field to pass an environment file to the Firelens sidecar container.

```yaml
# in copilot/{service name}/manifest.yml
env_file: log.env
```

And in `log.env` we could have
```
#This is a comment and will be ignored
LOG_LEVEL=debug
LOG_INFO=all
```
In a sidecar definition:
```yaml
sidecars:
  nginx:
    image: nginx:latest
    env_file: ./nginx.env
    port: 8080
```

In the logging container:
```yaml
logging:
  retention: 1
  destination:
    Name: cloudwatch
    region: us-west-2
    log_group_name: /copilot/logs/
    log_stream_prefix: copilot/
  env_file: ./logging.env
```

## How do I know the name of my DynamoDB table, S3 bucket, RDS database, etc?

When using the Copilot CLI to provision additional AWS resources such as DynamoDB tables, S3 buckets, databases, etc., any output values will be passed in as environment variables to your app. For more information, check out the [additional resources guide](./addons/workload.en.md).
