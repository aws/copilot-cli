# app delete
```bash
$ copilot app delete [flags]
```

## What does it do?

`copilot app delete` deletes all resources associated with an application.

## What are the flags?

```bash
    --env-profiles stringToString   Optional. Environments and the profile to use to delete the environment. (default [])
-h, --help                          help for delete
    --yes                           Skips confirmation prompt.
```

## Examples
Force delete the application with environments "test" and "prod".
```bash
$ copilot app delete --yes --env-profiles test=default,prod=prod-profile
```