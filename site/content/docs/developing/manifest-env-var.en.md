# Environment Variables in the Manifest

## Shell environment variables
It’s possible to use environment variables in your shell to populate values in your manifest files:

``` yaml
image:
  location: id.dkr.ecr.zone.amazonaws.com/project-name:${TAG}
```

Suppose the shell has `TAG=version01`, the manifest example will be resolved as
```yaml
image:
  location: id.dkr.ecr.zone.amazonaws.com/project-name:version01
```
When Copilot defines the container, it will use the image located at `id.dkr.ecr.zone.amazonaws.com/project-name` and with tag `version01`.

!!! Info
    At this moment, you can only substitute shell environment variables for fields that accept strings, including `String` (e.g., `image.location`), `Array of Strings` (e.g., `entrypoint`), or `Map` where the value type is `String` (e.g., `secrets`).

## Predefined variables
Predefined variables are reserved variables that will be resolved by Copilot when interpreting the manifest. Currently, available predefined environment variables include:

- COPILOT_APPLICATION_NAME
- COPILOT_ENVIRONMENT_NAME

```yaml
secrets:
   DB_PASSWORD: /copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/db_password
```

Copilot will substitute `${COPILOT_APPLICATION_NAME}` and `${COPILOT_ENVIRONMENT_NAME}` with the names of the application and the environment where the workload is deployed. For example, when you run
```
$ copilot svc deploy --app my-app --env test
```
to deploy the service to the `test` environment in your `my-app` application, Copilot will resolve `/copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/db_password` to `/copilot/my-app/test/secrets/db_password`. (For more information of secret injection, see [here](../developing/secrets.en.md)).
