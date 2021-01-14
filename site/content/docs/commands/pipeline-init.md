# pipeline init
```bash
$ copilot pipeline init [flags]
```

## What does it do?
`copilot pipeline init` creates a pipeline manifest for the services in your workspace, using the environments associated with the application.

## What are the flags?
```bash
-a, --app string                   Name of the application.
-e, --environments strings         Environments to add to the pipeline.
-b, --git-branch string            Branch used to trigger your pipeline.
-t, --github-access-token string   GitHub personal access token for your repository.
-u, --url string                   The repository URL to trigger your pipeline.
-h, --help                         help for init
```

## Examples
Create a pipeline for the services in your workspace.
```bash
$ copilot pipeline init \
--url https://github.com/gitHubUserName/myFrontendApp.git \
--github-access-token file://myGitHubToken \
--environments "test,prod" 
```