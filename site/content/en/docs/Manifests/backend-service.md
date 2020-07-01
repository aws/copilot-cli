---
title: "Backend Service"
linkTitle: "Backend Service"
weight: 2
---
List of all available properties for a `'Backend Service'` manifest.
```yaml
# Your service name will be used in naming your resources like log groups, ECS services, etc.
name: api

# Your service is reachable at "http://{{.Name}}.${COPILOT_SERVICE_DISCOVERY_ENDPOINT}:{{.Image.Port}}" but is not public.
type: Backend App

image:
  # Path to your service's Dockerfile.
  build: ./api/Dockerfile
  # Port exposed through your container to route traffic to it.
  port: 8080

  #Optional. Configuration for your container healthcheck.
  healthcheck:
    # The command the container runs to determine if it's healthy.
    command: ["CMD-SHELL", "curl -f http://localhost:8080 || exit 1"]
    interval: 10s     # Time period between healthchecks. Default is 10 if omitted.
    retries: 2        # Number of times to retry before container is deemed unhealthy. Default is 2 if omitted.
    timeout: 5s       # How long to wait before considering the healthcheck failed. Default is 5s if omitted.
    start_period: 0s  # Grace period within which to provide containers time to bootstrap before failed health checks count towards the maximum number of retries. Default is 0s if omitted.

# Number of CPU units for the task.
cpu: 256
# Amount of memory in MiB used by the task.
memory: 512
# Number of tasks that should be running in your service.
count: 1

variables:                    # Optional. Pass environment variables as key value pairs.
  LOG_LEVEL: info

secrets:                      # Optional. Pass secrets from AWS Systems Manager (SSM) Parameter Store.
  GITHUB_TOKEN: GITHUB_TOKEN  # The key is the name of the environment variable, the value is the name of the SSM      parameter.

# Optional. You can override any of the values defined above by environment.
environments:
  test:
    count: 2               # Number of tasks to run for the "test" environment.
```