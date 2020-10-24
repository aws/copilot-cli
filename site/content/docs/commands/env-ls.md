# env ls
```bash
$ copilot env ls [flags]
```

## What does it do?
`copilot env ls` lists all the environments in your application.

## What are the flags?
```bash
-h, --help          help for ls
    --json          Optional. Outputs in JSON format.
-a, --app string    Name of the application.
```
You can use the `--json` flag if you'd like to programmatically parse the results.

## Examples
Lists all the environments for the frontend application.
```bash
$ copilot env ls -a frontend
```

## What does it look like?

![Running copilot env ls](https://raw.githubusercontent.com/kohidave/copilot-demos/master/env-ls.svg?sanitize=true)