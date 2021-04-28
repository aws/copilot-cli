# app show
```bash
$ copilot app show [flags]
```

## What does it do?

`copilot app show` shows configuration, environments and services for an application.

## What are the flags?

```bash
-h, --help          help for show
    --json          Optional. Outputs in JSON format.
-n, --name string   Name of the application.
```

## Examples
Shows info about the application "my-app".
```bash
$ copilot app show -n my-app
```

## What does it look like?

![Running copilot app show](https://raw.githubusercontent.com/kohidave/copilot-demos/master/app-show.svg?sanitize=true)