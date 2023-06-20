# init
```console
$ copilot init
```

## What does it do? 
`copilot init` is your starting point if you want to deploy your container app on AWS App Runner or Amazon ECS on AWS Fargate. Run it within a directory with your Dockerfile, and `init` will ask you questions about your application so we can get it up and running quickly.

After you answer all the questions, `copilot init` will set up an ECR repository for you and ask you if you'd like to deploy. If you opt to deploy, it'll create a new `test` environment (complete with a networking stack and roles), build your Dockerfile, push it to Amazon ECR, and deploy your service or job. 

If you have an existing app, and want to add another service or job to that app, you can run `copilot init` - and you'll be prompted to select an existing app to add your service or job to. 

## What are the flags?

Like all commands in the Copilot CLI, if you don't provide required flags, we'll prompt you for all the information we need to get you going. You can skip the prompts by providing information via flags:

```
  -a, --app string          Name of the application.
      --deploy              Deploy your service or job to a "test" environment.
  -d, --dockerfile string   Path to the Dockerfile.
                            Mutually exclusive with -i, --image.
  -h, --help                help for init
  -i, --image string        The location of an existing Docker image.
                            Mutually exclusive with -d, --dockerfile.
  -n, --name string         Name of the service or job.
      --port uint16         Optional. The port on which your service listens.
      --retries int         Optional. The number of times to try restarting the job on a failure.
      --schedule string     The schedule on which to run this job. 
                            Accepts cron expressions of the format (M H DoM M DoW) and schedule definition strings. 
                            For example: "0 * * * *", "@daily", "@weekly", "@every 1h30m".
                            AWS Schedule Expressions of the form "rate(10 minutes)" or "cron(0 12 L * ? 2021)"
                            are also accepted.
      --tag string          Optional. The tag for the container images Copilot builds from Dockerfiles.
      --timeout string      Optional. The total execution time for the task, including retries.
                            Accepts valid Go duration strings. For example: "2h", "1h30m", "900s".
  -t, --type string         Type of service to create. Must be one of:
                            "Request-Driven Web Service", "Load Balanced Web Service", "Backend Service", "Scheduled Job".
```