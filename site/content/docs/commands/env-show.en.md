# env show
```console
$ copilot env show [flags]
```

## What does it do?
`copilot env show` displays information about a particular environment, including:

* The region and account the environment is in  
* The services currently deployed in the environment  
* The tags associated with that environment  

You can optionally pass in a `--resources` flag which will include the AWS resources associated specifically with the environment. 

## What are the flags?
```
-a, --app string    Name of the application.
-h, --help          help for show
    --json          Optional. Output in JSON format.
    --manifest      Optional. Output the manifest file used for the deployment.
-n, --name string   Name of the environment.
    --resources     Optional. Show the resources in your environment.
```
You can use the `--json` flag if you'd like to programmatically parse the results.

## Examples
Print configuration for the "test" environment.
```console
$ copilot env show -n test
```
Print manifest file for deploying the "prod" environment.
```console
$ copilot env show -n prod --manifest
```
