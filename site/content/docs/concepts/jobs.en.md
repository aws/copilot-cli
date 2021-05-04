Jobs are Amazon ECS tasks that are triggered by an event. Currently, Copilot supports only "Scheduled Jobs".
These are tasks that can be triggered either on a fixed schedule or periodically by providing a rate.

## Creating a Job

The easiest way to create a job is to run the `init` command from the same directory as your Dockerfile.

```bash
$ copilot init
```

Once you select which application the job should be part of, Copilot will ask you the __type__ of
job you'd like to create. Currently, Copilot only supports "Scheduled Job".

## Config and the Manifest

After you've run `copilot init`, the CLI will create a file called `manifest.yml`.
The [scheduled job manifest](../manifest/scheduled-job.en.md) is a simple declarative file that 
contains the most common configuration for a task that's triggered by a scheduled event. For example,
you can configure when you'd like to trigger the job, the container size, the timeout for the task, as well as
how many times to retry in case of failures.

## Deploying a Job

Once you've configured your manifest file to satisfy your requirements, you can deploy the changes with the deploy command:
```bash
$ copilot deploy
```

Running this command will:

1. Build your image locally  
2. Push to your job's ECR repository  
3. Convert the manifest file to a CloudFormation template  
4. Package any additional infrastructure into the CloudFormation template  
5. Deploy your resources

### What's in a Job?

Since Copilot uses CloudFormation under the hood, all the resources created are visible and tagged by Copilot.
Scheduled Jobs are composed of an AmazonECS Task Definition, Task Role, Task Execution Role, 
a Step Function State Machine for retrying on failures, and finally an Event Rule to trigger the state machine.

 