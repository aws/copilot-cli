List of all available properties for a `'Load Balanced Web Service'` manifest. To learn about Copilot services, see the [Services](../concepts/services.en.md) concept page.

???+ note "Sample manifest for a frontend service"

    ```yaml
    # Your service name will be used in naming your resources like log groups, ECS services, etc.
    name: frontend
    type: Load Balanced Web Service

    # Distribute traffic to your service.
    http:
      path: '/'
      healthcheck:
        path: '/_healthcheck'
        success_codes: '200,301'
        healthy_threshold: 3
        unhealthy_threshold: 2
        interval: 15s
        timeout: 10s
      stickiness: false
      allowed_source_ips: ["10.24.34.0/23"]
      alias: example.com

    # Configuration for your containers and service.
    image:
      build:
        dockerfile: ./frontend/Dockerfile
        context: ./frontend
      port: 80

    cpu: 256
    memory: 512
    count:
      range: 1-10
      cpu_percentage: 70
      memory_percentage: 80
      requests: 10000
      response_time: 2s
    exec: true

    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # You can override any of the values defined above by environment.
    environments:
      test:
        count:
          range:
            min: 1
            max: 10
            spot_from: 2
      staging:
        count:
          spot: 2
      production:
        count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. A [Load Balanced Web Service](../concepts/services.en.md#load-balanced-web-service) is an internet-facing service that's behind a load balancer, orchestrated by Amazon ECS on AWS Fargate.

{% include 'http-config.en.md' %}

{% include 'image-config.en.md' %}

{% include 'image-healthcheck.en.md' %}

{% include 'common-svc-fields.en.md' %}
