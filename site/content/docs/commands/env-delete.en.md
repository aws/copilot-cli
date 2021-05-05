# env delete
```bash
$ copilot env delete [flags]
```

## What does it do?
`copilot env delete` deletes an environment from your application. If there are running applications in your environment, you need to first run [`copilot svc delete`](../commands/svc-delete.en.md).

After you answer the questions, you should see that the AWS CloudFormation stack for your environment has been deleted.

## What are the flags?
```
-h, --help             help for delete
-n, --name string      Name of the environment.
    --yes              Skips confirmation prompt.
-a, --app string       Name of the application.
```

## Examples
Delete the "test" environment.
```bash
$ copilot env delete --name test 
```
Delete the "test" environment without prompting.
```bash
$ copilot env delete --name test --yes
```
