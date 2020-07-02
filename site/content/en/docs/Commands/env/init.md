---
title: "env init"
linkTitle: "env init"
weight: 1
---
```bash
$ copilot env init [flags]
```

### What does it do?
`copilot env init` creates a new [environment](docs/concepts/environments) where your services will live.

After you answer the questions, the CLI creates the common infrastructure that's shared between your services such as a VPC, an Application Load Balancer, and an ECS Cluster.

You create environments using a [named profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) to specify which AWS Account and Region you'd like the environment to be in.

### What are the flags?
Like all commands in the AWS Copilot CLI, if you don't provide required flags, we'll prompt you for all the information we need to get you going. You can skip the prompts by providing information via flags:
```
-h, --help             help for init
-n, --name string      Name of the environment.
    --prod             If the environment contains production services.
    --profile string   Name of the profile.
-a, --app string       Name of the application.
```

### Examples
Creates a test environment in your "default" AWS profile.
```bash
$ copilot env init --name test --profile default
```

Creates a prod-iad environment using your "prod-admin" AWS profile.
```bash
$ copilot env init --name prod-iad --profile prod-admin --prod
```

### What does it look like?
<img class="img-fluid" src="https://raw.githubusercontent.com/kohidave/copilot-demos/master/env-init.svg?sanitize=true" style="margin-bottom: 20px;">
