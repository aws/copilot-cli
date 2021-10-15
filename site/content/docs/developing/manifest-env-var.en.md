# Environment Variables in Manifest

There are multiple scenarios that you might want to use environment variables in your [manifest](../manifest/overview.en.md). This page should help you find the information you need.

## Shell environment variables
It’s possible to use environment variables in your shell to populate values in your manifest files:

``` yaml
name: my-service
type: Load Balanced Web Service
image:
  location: id.dkr.ecr.zone.amazonaws.com/project-name:${TAG}
  port: 3333
```

!!! Info
    At this stage, the value of an environment variable needs strictly to be a `string`. For example, Copilot won't automatically convert, let say "8080", to 8080 for your `port`.

## Predefined variables
Predefined variables are reserved environment variables resolved by Copilot before your manifest deployment:

```yaml
secrets:
   DB_PASSWORD: /copilot/my-app/${COPILOT_ENVIRONMENT_NAME}/secrets/db_password
environments:
    prod:
      secrets:
          <specific environment variable for prod>
    dev:
      secrets:
          <specific environment variable for dev>
```

Currently, available predefined environment variables include:

- COPILOT_APPLICATION_NAME
- COPILOT_ENVIRONMENT_NAME

!!! Alert
    :warning: Predefined variables are not allowed to be overridden.
