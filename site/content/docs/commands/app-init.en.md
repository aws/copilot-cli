# app init
```console
$ copilot app init [name] [flags]
```

## What does it do?
`copilot app init` creates a new [application](../concepts/applications.en.md) within the directory that will contain your service(s).

After you answer the questions, the CLI creates AWS Identity and Access Management roles to manage the release infrastructure for your services. You'll also see a new sub-directory created under your working directory: `copilot/`. The `copilot` directory will hold the manifest files and additional infrastructure for your services.

Typically, you don't need to run `app init` (`init` does all the same work) unless you want to use a custom domain name or AWS tags, or pass in an IAM policy for a permissions boundary. 

## What are the flags?
Like all commands in the Copilot CLI, if you don't provide required flags, we'll prompt you for all the information we need to get you going. You can skip the prompts by providing information via flags:
```
      --domain string                  Optional. Your existing custom domain name.
  -h, --help                           help for init
      --permissions-boundary           Optional. The name or ARN of an existing IAM policy with which to set a
                                       permissions boundary for all roles generated within the application.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
```
The `--domain` flag allows you to specify a domain name registered with Amazon Route 53 in your app's account. This will allow all the services in your app to share the same domain name. You'll be able to access your services at: [https://{svcName}.{envName}.{appName}.{domain}](https://{svcName}.{envName}.{appName}.{domain})

The `--permissions-boundary` flag allows you to indicate an existing IAM policy in your app's account. This policy name will become part of an ARN to add permissions boundaries to all Copilot-created IAM roles in your app.

The `--resource-tags` flags allows you to add your custom [tags](https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html) to all the resources in your app.
For example: `copilot app init --resource-tags department=MyDept,team=MyTeam`

## Examples
Create a new application named "my-app".
```console
$ copilot app init my-app
```
Create a new application with an existing domain name in Amazon Route53.
```console
$ copilot app init --domain example.com
```
Create a new application with resource tags.
```console
$ copilot app init --resource-tags department=MyDept,team=MyTeam
```
## What does it look like?

![Running copilot app init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/app-init.edited.svg?sanitize=true)
