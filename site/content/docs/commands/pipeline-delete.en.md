# pipeline delete
```bash
$ copilot pipeline delete [flags]
```

## What does it do?
`copilot pipeline delete` deletes the pipeline associated with your workspace.

## What are the flags?
```bash
-a, --app             Name of the application.
    --delete-secret   Deletes AWS Secrets Manager secret associated with a pipeline source repository.
-h, --help            help for delete
-n, --name            Name of the pipeline.
    --yes             Skips confirmation prompt.
```

## Examples
Delete the pipeline associated with your workspace.
```bash
$ copilot pipeline delete
```