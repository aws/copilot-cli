# job init
```console
$ copilot job init
```

## What does it do?

`copilot job init` creates a new [job](../concepts/jobs.en.md) to run your code for you. 

After running this command, the CLI creates sub-directory with your app name in your local `copilot` directory where you'll find a [manifest file](../manifest/overview.en.md). Feel free to update your manifest file to change the default configs for your job. The CLI also sets up an ECR repository with a policy for all [environments](../concepts/environments.en.md) to be able to pull from it. Then, your job gets registered to AWS System Manager Parameter Store so that the CLI can keep track of it for you.

After that, if you already have an environment set up, you can run `copilot job deploy` to deploy your job in that environment.

## What are the flags?

```
      --allow-downgrade     Optional. Allow using an older version of Copilot to update Copilot components
                            updated by a newer version of Copilot.
  -a, --app string          Name of the application.
  -d, --dockerfile string   Path to the Dockerfile.
                            Mutually exclusive with -i, --image.
  -h, --help                help for init
  -i, --image string        The location of an existing Docker image.
                            Mutually exclusive with -d, --dockerfile.
  -t, --job-type string     Type of job to create. Must be one of:
                            "Scheduled Job".
  -n, --name string         Name of the job.
      --retries int         Optional. The number of times to try restarting the job on a failure.
  -s, --schedule string     The schedule on which to run this job. 
                            Accepts cron expressions of the format (M H DoM M DoW) and schedule definition strings. 
                            For example: "0 * * * *", "@daily", "@weekly", "@every 1h30m".
                            AWS Schedule Expressions of the form "rate(10 minutes)" or "cron(0 12 L * ? 2021)"
                            are also accepted.
      --timeout string      Optional. The total execution time for the task, including retries.
                            Accepts valid Go duration strings. For example: "2h", "1h30m", "900s".
```

## Examples

 Creates a "reaper" scheduled task to run once per day.
```console
$ copilot job init --name reaper --dockerfile ./frontend/Dockerfile --schedule "@daily"
```
Creates a "report-generator" scheduled task with retries.
```console
$ copilot job init --name report-generator --schedule "@monthly" --retries 3 --timeout 900s
```
