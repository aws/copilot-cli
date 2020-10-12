# init
```bash
$ copilot init
```

## What does it do? 
`copilot init` is your starting point if you want to deploy your container app on Amazon ECS. Run it within a directory with your Dockerfile, and `init` will ask you questions about your application so we can get it up and running quickly. 

After you answer all the questions, `copilot init` will set up an ECR repository for you and ask you if you'd like to deploy. If you opt to deploy, it'll create a new `test` environment (complete with a networking stack and roles), build your Dockerfile, push it to Amazon ECR, and deploy your service. 

If you have an existing app, and want to add another service to that app, you can run `copilot init` - and you'll be prompted to select an existing app to add your app to. 

## What are the flags?

Like all commands in the copilot CLI, if you don't provide required flags, we'll prompt you for all the information we need to get you going. You can skip the prompts by providing information via flags:

```sh
  -a, --app string          Name of the application.
      --deploy              Deploy your service to a "test" environment.
  -d, --dockerfile string   Path to the Dockerfile.
  -h, --help                help for init
      --port uint16         Optional. The port on which your service listens.
      --profile string      Name of the profile. (default "default")
  -s, --svc string          Name of the service.
  -t, --svc-type string     Type of service to create. Must be one of:
                            "Load Balanced Web Service", "Backend Service"
      --tag string          Optional. The service's image tag.
```

## What does it look like?
![Running copilot init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/init-no-deploy.svg?sanitize=true)