---
title: "env show"
linkTitle: "env show"
weight: 3
---

```bash
$ copilot env show [flags]
```

### What does it do?
`copilot env show` shows info about a deployed environment, including region, account ID, and services.

### What are the flags?
```bash
-h, --help          help for show
    --json          Optional. Outputs in JSON format.
-n, --name string   Name of the environment.
    --resources     Optional. Show the resources in your environment.
```
You can use the `--json` flag if you'd like to programmatically parse the results.

### Examples
Shows info about the environment "test".
```bash
$ copilot env show -n test
```
