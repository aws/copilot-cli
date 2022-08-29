Jobs are Amazon ECS tasks that are triggered by an event. Currently, Copilot supports only "Scheduled Jobs".
These are tasks that can be triggered either on a fixed schedule or periodically by providing a rate.

## Creating a Job

The easiest way to create a job is to run the `init` command from the same directory as your Dockerfile.

```console
$ copilot init
```

Once you select which application the job should be part of, Copilot will ask you the __type__ of
job you'd like to create. Currently, Copilot only supports "Scheduled Job".

## Config and the Manifest

After you've run `copilot init`, the CLI will create a file called `manifest.yml` in the `copilot/[job name]/` directory.
The [scheduled job manifest](../manifest/scheduled-job.en.md) is a simple declarative file that 
contains the most common configuration for a task that's triggered by a scheduled event. For example,
you can configure when you'd like to trigger the job, the container size, the timeout for the task, as well as
how many times to retry in case of failures.

## Deploying a Job

Once you've configured your manifest file to satisfy your requirements, you can deploy the changes with the deploy command:
```console
$ copilot deploy
```

Running this command will:

1. Build your image locally  
2. Push to your job's ECR repository  
3. Convert the manifest file to a CloudFormation template  
4. Package any additional infrastructure into the CloudFormation template  
5. Deploy your resources

## Other Job-Related Options

Say you want to give a new job a test spin or want to invoke it for some other reason. Use the [`job run`](../commands/job-run.en.md) command: 
```console
$ copilot job run
```

If you'd like to temporarily disable your job without deleting it entirely, set the schedule to `none` in your [manifest](../manifest/scheduled-job.en.md):
```yaml
on:
  schedule: "none"
```

To print out the CloudFormation template for a configured job, run [`job package`](../commands/job-package.en.md):
```console
$ copilot job package
```

### What's in a Job?

Since Copilot uses CloudFormation under the hood, all the resources created are visible and tagged by Copilot.
Scheduled Jobs are composed of an AmazonECS Task Definition, Task Role, Task Execution Role, 
a Step Function State Machine for retrying on failures, and finally an Event Rule to trigger the state machine.

### Where Are My Job's Logs?

Checking your job logs is easy as well. Running [`copilot job logs`](../commands/job-logs.en.md) will show the most recent logs of your job. You can follow your logs live with the `--follow` flag,
which will display logs from any new invocation of your job after you run the command.

```console
$ copilot job logs
copilot/myjob/37236ed Doing some work
copilot/myjob/37236ed Did some work
copilot/myjob/37236ed Exited...
copilot/myjob/123e300 Doing some work
copilot/myjob/123e300 Did some work
copilot/myjob/123e300 Did some additional work
copilot/myjob/123e300 Exited
```
