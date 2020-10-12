# pipeline init
```bash
$ copilot pipeline init [flags]
```

## What does it do?
`copilot pipeline init` creates a pipeline manifest for the services in your workspace, using the environments associated with the application.

## What are the flags?
```bash
-e, --environments strings         Environments to add to the pipeline.
-b, --git-branch string            Branch used to trigger your pipeline.
-t, --github-access-token string   GitHub personal access token for your repository.
-u, --github-url string            GitHub repository URL for your service.
-h, --help                         help for init
```

## Examples
Create a pipeline for the services in your workspace.
```bash
$ copilot pipeline init \
--github-url https://github.com/gitHubUserName/myFrontendApp.git \
--github-access-token file://myGitHubToken \
--environments "test,prod" 
```