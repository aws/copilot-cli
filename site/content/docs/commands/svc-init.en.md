# svc init
```console
$ copilot svc init
```

## What does it do?

`copilot svc init` creates a new [service](../concepts/services.en.md) to run your code for you. 

After running this command, the CLI creates sub-directory with your app name in your local `copilot` directory where you'll find a [manifest file](../manifest/overview.en.md). Feel free to update your manifest file to change the default configs for your service. The CLI also sets up an ECR repository with a policy for all [environments](../concepts/environments.en.md) to be able to pull from it. Then, your service gets registered to AWS System Manager Parameter Store so that the CLI can keep track of it.

After that, if you already have an environment set up, you can run `copilot deploy` to deploy your service in that environment.

## What are the flags?

```
Flags
  -a, --app string          Name of the application.
  -d, --dockerfile string   Path to the Dockerfile.
                            Mutually exclusive with -i, --image.
  -i, --image string        The location of an existing Docker image.
                            Mutually exclusive with -d, --dockerfile.
  -n, --name string         Name of the service.
      --port uint16         The port on which your service listens.
  -t, --svc-type string     Type of service to create. Must be one of:
                            "Request-Driven Web Service", "Load Balanced Web Service", "Backend Service", "Worker Service".
```

To create a "frontend" load balanced web service you could run:

`$ copilot svc init --name frontend --svc-type "Load Balanced Web Service" --dockerfile ./frontend/Dockerfile`

## What does it look like?

![Running copilot svc init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-init.svg?sanitize=true)
