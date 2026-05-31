# app delete
```console
$ copilot app delete [flags]
```

## What does it do?

`copilot app delete` deletes all resources associated with an application.

## What are the flags?

```
-h, --help          help for delete
-n, --name string   Name of the application.
    --yes           Skips confirmation prompt.
```

## Examples
!!!warning "Use `--name` for application names"
    `copilot app delete` does not accept a positional application name. To delete a specific application, pass the name with `--name`.

Force delete an application named "phonetool".
```console
$ copilot app delete --name phonetool --yes
```
