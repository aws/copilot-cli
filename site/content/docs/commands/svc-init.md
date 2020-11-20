# svc init
```bash
$ copilot svc init
```

## What does it do?

`copilot svc init` creates a new [service](../concepts/services.md) to run your code for you. 

After running this command, the CLI creates sub-directory with your app name in your local `copilot` directory where you'll find a [manifest file](../manifest/overview.md). Feel free to update your manifest file to change the default configs for your service. The CLI also sets up an ECR repository with a policy for all [environments](../concepts/environments.md) to be able to pull from it. Then, your service gets registered to AWS System Manager Parameter Store so that the CLI can keep track of it.

After that, if you already have an environment set up, you can run `copilot deploy` to deploy your service in that environment.

## What are the flags?

```bash
Required Flags
  -d, --dockerfile string   Path to the Dockerfile.
  -n, --name string         Name of the service.
  -t, --svc-type string     Type of service to create. Must be one of:
                            "Load Balanced Web Service", "Backend Service"

Load Balanced Web Service Flags
      --port uint16   Optional. The port on which your service listens.

Backend Service Flags
      --port uint16   Optional. The port on which your service listens.
```

Each service type has its own optional and required flags besides the common required flags.
To create a "frontend" load balanced web service you could run:  

`$ copilot svc init --name frontend --app-type "Load Balanced Web Service" --dockerfile ./frontend/Dockerfile`

## What does it look like?

![Running copilot svc init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-init.svg?sanitize=true)
