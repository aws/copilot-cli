# env show
```bash
$ copilot env show [flags]
```

## What does it do?
`copilot env show` shows displays information about a particular environment, including:

* The region and account the environment is in  
* Whether or not the environment is production  
* The services currently deployed in the environment  
* The tags associated with that environment  

You can optionally pass in a `--resources` flag which will include the AWS resources associated specifically with the environment. 

## What are the flags?
```bash
-h, --help          help for show
    --json          Optional. Outputs in JSON format.
-n, --name string   Name of the environment.
    --resources     Optional. Show the resources in your environment.
```
You can use the `--json` flag if you'd like to programmatically parse the results.

## Examples
Shows info about the environment "test".
```bash
$ copilot env show -n test
```