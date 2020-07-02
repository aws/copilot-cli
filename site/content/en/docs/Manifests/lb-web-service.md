---
title: "Load Balanced Web Service"
linkTitle: "Load Balanced Web Service"
weight: 1
---
List of all available properties for a `'Load Balanced Web Service'` manifest.
```yaml
# Your service name will be used in naming your resources like log groups, ECS services, etc.
name: frontend
# The "architecture" of the service you're running.
type: Load Balanced Web Service

image:
  # Path to your service's Dockerfile.
  build: ./Dockerfile
  # Port exposed through your container to route traffic to it.
  port: 80

http:
  # Requests to this path will be forwarded to your service. 
  # To match all requests you can use the "/" path. 
  path: '/'
  # You can specify a custom health check path. The default is "/"
  # healthcheck: "/"

# Number of CPU units for the task.
cpu: 256
# Amount of memory in MiB used by the task.
memory: 512
# Number of tasks that should be running in your service.
count: 1

variables:                    # Optional. Pass environment variables as key value pairs.
  LOG_LEVEL: info

secrets:                      # Optional. Pass secrets from AWS Systems Manager (SSM) Parameter Store.
  GITHUB_TOKEN: GITHUB_TOKEN  # The key is the name of the environment variable, the value is the name of the SSM parameter.


# Optional. You can override any of the values defined above by environment.
environments:
  test:
    count: 2               # Number of tasks to run for the "test" environment.
```