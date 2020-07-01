---
title: "Environment Variables"
linkTitle: "Environment Variables"
weight: 1
---
Environment variables are variables that are available to your service, based on the environment they're running in. Your service can reference them without having to define them. Environment variables are useful for when you want to pass in data to your service that's specific to a particular environment (your test database name versus your production database name, for example). 

Accessing environment variables is usually simply based on the language you're using. Here are some examples of getting an environment variable called `DATABASE_NAME` in a few different languages. 

__Go__
```golang
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

### What are the default Environment Variables?

By default, the Copilot CLI passes in some default environment variables for your app to use. 

* `COPILOT_APPLICATION_NAME` - this is the name of the application this service is running in. 
* `COPILOT_ENVIRONMENT_NAME` - this is the name of the environment the service is running in (test vs prod, for example)
* `COPILOT_SERVICE_NAME` - this is the name of the current service. 
* `COPILOT_LB_DNS` - this is the DNS name of the Load Balancer (if it exists) such as _kudos-Publi-MC2WNHAIOAVS-588300247.us-west-2.elb.amazonaws.com_. One note, if you're using a custom domain name, this value will still be the Load Balancer's DNS name. 
* `COPILOT_SERVICE_DISCOVERY_ENDPOINT` - this is the endpoint to add after a service name to talk to another service in your environment via service discovery. The value is `{app name}.local`. For more information about service discovery checkout our [service discovery guide](docs/developing/service-discovery).

### How do I add my own Environment Variables?

Adding your own environment variable is easy. You can add them directly to your [manifest](docs/manifests) in the `variables` section. The following snippet will pass a environment variable called `LOG_LEVEL` to your service, with the value set to `debug`. 

```yaml
# in copilot/{service name}/manifest.yml 
variables:                    
  LOG_LEVEL: debug
```

You can also pass in a specific value for an environment variable based on the environment. We'll follow the same example as above, by setting the log level, but overwriting the value to be `info` in our production environment. Changes to your manifest take effect when you deploy them, so changing them locally is safe.

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

<img src="https://raw.githubusercontent.com/kohidave/ecs-cliv2-demos/master/env-vars-edit.svg?sanitize=true" class="img-fluid" style="margin-bottom: 20px;">

### How do I know the name of my DynamoDB table, S3 bucket, RDS database, etc?

When using the Copilot CLI to provision additional AWS resources, such as DynamoDB tables, S3 buckets, Databases, etc, any output values will be passed in as environment variables to your app. For more information, check out the [additional resources guide](docs/developing/addons). 
