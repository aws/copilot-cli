# pipeline delete
```bash
$ copilot pipeline delete [flags]
```

## What does it do?
`copilot pipeline delete` deletes the pipeline associated with your workspace.

## What are the flags?
```bash
    --delete-secret   Deletes AWS Secrets Manager secret associated with a pipeline source repository.
-h, --help            help for delete
    --yes             Skips confirmation prompt.
```

## Examples
Delete the pipeline associated with your workspace.
```bash
$ copilot pipeline delete
```