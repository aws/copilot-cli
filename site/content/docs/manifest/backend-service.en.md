List of all available properties for a `'Backend Service'` manifest. To learn about Copilot services, see the [Services](../concepts/services.en.md) concept page.

???+ note "Sample manifest for an api service"

    ```yaml
    # Your service name will be used in naming your resources like log groups, ECS services, etc.
    name: api
    type: Backend Service

    # Your service is reachable at "http://api.${COPILOT_SERVICE_DISCOVERY_ENDPOINT}:8080" but is not public.

    # Configuration for your containers and service.
    image:
      build: ./api/Dockerfile
      port: 8080
      healthcheck:
        command: ["CMD-SHELL", "curl -f http://localhost:8080 || exit 1"]
        interval: 10s
        retries: 2
        timeout: 5s
        start_period: 0s

    cpu: 256
    memory: 512
    count: 1
    exec: true

    storage:
      volumes:
        myEFSVolume:
          path: '/etc/mount1'
          read_only: true
          efs:
            id: fs-12345678
            root_dir: '/'
            auth:
              iam: true
              access_point_id: fsap-12345678

    network:
      vpc:
        placement: 'private'
        security_groups: ['sg-05d7cd12cceeb9a6e']

    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # You can override any of the values defined above by environment.
    environments:
      test:
        count:
          spot: 2
      production:
        count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. [Backend Services](../concepts/services.en.md#backend-service) are not reachable from the internet, but can be reached with [service discovery](../developing/service-discovery.en.md) from your other services.

{% include 'image-config.en.md' %}

{% include 'image-healthcheck.en.md' %}

{% include 'common-svc-fields.en.md' %}

{% include 'publish.en.md' %}
