# svc init
```console
$ copilot svc init
```

## What does it do?

`copilot svc init` creates a new [service](../concepts/services.en.md) to run your code for you. 

After running this command, the CLI creates sub-directory with your service name in your local `copilot` directory where you'll find a [manifest file](../manifest/overview.en.md). Feel free to update your manifest file to change the default configs for your service. The CLI also sets up an ECR repository with a policy for all [environments](../concepts/environments.en.md) to be able to pull from it. Then, your service gets registered to AWS System Manager Parameter Store so that the CLI can keep track of it.

After that, if you already have an environment set up, you can run `copilot deploy` to deploy your service in that environment.

## What are the flags?

```
Flags
      --allow-downgrade                Optional. Allow using an older version of Copilot to update Copilot components
                                       updated by a newer version of Copilot.
  -a, --app string                     Name of the application.
  -d, --dockerfile string              Path to the Dockerfile.
                                       Cannot be specified with --image.
  -h, --help                           help for init
  -i, --image string                   The location of an existing Docker image.
                                       Cannot be specified with --dockerfile or --build-context.
      --ingress-type string            Required for a Request-Driven Web Service. Allowed source of traffic to your service.
                                       Must be one of Environment or Internet.
  -n, --name string                    Name of the service.
      --no-subscribe                   Optional. Turn off selection for adding subscriptions for worker services.
      --port uint16                    The port on which your service listens.
      --sources stringArray            List of relative paths to source directories or files.
                                       Must be specified with '--svc-type "Static Site"'.
      --subscribe-topics stringArray   Optional. SNS topics to subscribe to from other services in your application.
                                       Must be of format '<svcName>:<topicName>'.
  -t, --svc-type string                Type of service to create. Must be one of:
                                       "Request-Driven Web Service", "Load Balanced Web Service", "Backend Service", "Worker Service", "Static Site".
```

To create a "frontend" load balanced web service you could run:

`$ copilot svc init --name frontend --svc-type "Load Balanced Web Service" --dockerfile ./frontend/Dockerfile`

## What does it look like?

![Running copilot svc init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-init.svg?sanitize=true)
