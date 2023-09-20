# Sidecars
Sidecars are additional containers that run along side the main container. They are usually used to perform peripheral tasks such as logging, configuration, or proxying requests.

!!! Attention
    Sidecars are not supported for Request-Driven Web Services.  

!!! Attention
    If your main container is using a Windows image, [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html), [AWS X-Ray](https://aws.amazon.com/xray/), and [AWS App Mesh](https://aws.amazon.com/app-mesh/) are not supported. Please check if your sidecar container supports Windows.


AWS also provides some plugin options that can be seamlessly incorporated with your ECS service, including but not limited to [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html), [AWS X-Ray](https://aws.amazon.com/xray/), and [AWS App Mesh](https://aws.amazon.com/app-mesh/).

If you have defined an EFS volume for your main container through the [`storage` field](../developing/storage.en.md) in the manifest, you can also mount that volume in any sidecar containers you have defined.

## How to add sidecars with Copilot?
There are two ways of adding sidecars using the Copilot manifest: by specifying [general sidecars](#general-sidecars) or by using [sidecar patterns](#sidecar-patterns).

### General sidecars
You'll need to provide the URL for the sidecar image. Optionally, you can specify the port you'd like to expose and the credential parameter for [private registry](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html).

{% include 'sidecar-config.en.md' %}

<div class="separator"></div>

#### Example

##### Sidecars with environment overrides
Similar to other service/job manifest fields, sidecars configurations can be overridden per-environment via the [`environments`](../manifest/lb-web-service.en.md#environments) field.
Below is an example that configures the value for the `DD_APM_ENABLED` environment variable of the `datadog` sidecar, based on whether it is `dev` environment:

```yaml
name: api
type: Load Balanced Web Service

sidecars:
  datadog:
    port: 80
    image:
      build: src/reverseproxy/Dockerfile
    variables:
      DD_APM_ENABLED: true

environments:
  dev:
    sidecars:
      datadog:
        variables:
          DD_APM_ENABLED: false
```

##### [nginx](https://www.nginx.com/) sidecar container
Below is an example of specifying the [nginx](https://www.nginx.com/) sidecar container in a load balanced web service manifest.

``` yaml
name: api
type: Load Balanced Web Service

image:
  build: api/Dockerfile
  port: 3000

http:
  path: 'api'
  healthcheck: '/api/health-check'
  # Target container for Load Balancer is our sidecar 'nginx', instead of the service container.
  target_container: 'nginx'

cpu: 256
memory: 512
count: 1

sidecars:
  nginx:
    port: 80
    image:
      build: src/reverseproxy/Dockerfile
    variables:
      NGINX_PORT: 80
```

##### EFS volume in both the service and sidecar container

```yaml
storage:
  volumes:
    myEFSVolume:
      path: '/etc/mount1'
      read_only: false
      efs:
        id: fs-1234567

sidecars:
  nginx:
    port: 80
    image: 1234567890.dkr.ecr.us-west-2.amazonaws.com/reverse-proxy:revision_1
    variables:
      NGINX_PORT: 80
    mount_points:
      - source_volume: myEFSVolume
        path: '/etc/mount1'
```
##### [AWS Distro for OpenTelemetry](https://aws-otel.github.io/) sidecar
Below is an example of running the [AWS Distro for OpenTelemetry](https://aws-otel.github.io/) sidecar with a custom configuration. The example
custom configuration will not only collect X-Ray trace data, but also ship ECS metrics to a third party. The example will require an SSM secret and additional IAM permissions.

To use the OpenTelemetry sidecar, first, create a valid [configuration file](https://opentelemetry.io/docs/collector/configuration/). Next, check the size of the configuration file.  A standard parameter is [limited to 4KB](https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_PutParameter.html#systemsmanager-PutParameter-request-Value).
If the configuration file is larger than 4K, an advanced SSM parameter must be used.

If an advanced parameter is required, it will need to be created and tagged manually.  If the configuration fits within a standard parameter, create an SSM secret using the [`secret init`](../commands/secret-init.en.md). The YAML document below can be used as-is with New Relic after updating the API key written as "YOUR-API-KEY-HERE".

In the example YAML, the inclusion of empty keys is deliberate. The sidecar will use the collector defaults for those keys.


```yaml
receivers:
  awsxray:
    transport: udp
  awsecscontainermetrics:

processors:
  batch:

exporters:
  awsxray:
    region: us-west-2
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers: 
      api-key: YOUR-API-KEY-HERE

service:
  pipelines:
    traces:
      receivers: [awsxray]
      processors: [batch]
      exporters: [awsxray]
    metrics:
      receivers: [awsecscontainermetrics]
      exporters: [otlp]
```

Writing X-Ray traces needs additional IAM permissions as shown below. Include this in addons according to the [published documentation](./addons/workload.en.md)

``` yaml
Resources:
  XrayWritePolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
          - Sid: CopyOfAWSXRayDaemonWriteAccess
            Effect: Allow
            Action:
              - xray:PutTraceSegments
              - xray:PutTelemetryRecords
              - xray:GetSamplingRules
              - xray:GetSamplingTargets
              - xray:GetSamplingStatisticSummaries
            Resource: "*"

Outputs:
  XrayAccessPolicyArn:
    Description: "The ARN of the ManagedPolicy to attach to the task role."
    Value: !Ref XrayWritePolicy
```

The configuration for the OpenTelemetry collector will be passed into the sidecar as an environment variable.

```yaml
sidecars:
  otel_sidecar:
    image: 'public.ecr.aws/aws-observability/aws-otel-collector:latest'
    secrets:
      AOT_CONFIG_CONTENT: /copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/otel_config
```

### Sidecar patterns
Sidecar patterns are predefined Copilot sidecar configurations. For now, the only supported pattern is FireLens, but we'll add more in the future!

``` yaml
# In the manifest.
logging:
  # The Fluent Bit image. (Optional, we'll use "public.ecr.aws/aws-observability/aws-for-fluent-bit:stable" by default)
  image: <image URL>
  # The configuration options to send to the FireLens log driver. (Optional)
  destination:
    <config key>: <config value>
  # Whether to include ECS metadata in logs. (Optional, default to true)
  enableMetadata: <true|false>
  # Secret to pass to the log configuration. (Optional)
  secretOptions:
    <key>: <value>
  # The full config file path in your custom Fluent Bit image. (Optional)
  configFilePath: <config file path>
  # Environment variables for the sidecar container. (Optional)
  variables:
    <key>: <value>
  # Secrets to expose to the sidecar container. (Optional)
  secrets:
    <key>: <value>
```
For example:

``` yaml
logging:
  destination:
    Name: cloudwatch
    region: us-west-2
    log_group_name: /copilot/sidecar-test-hello
    log_stream_prefix: copilot/
```

You might need to add necessary permissions to the task role so that FireLens can forward your data. You can add permissions by specifying them in your [addons](./addons/workload.en.md). For example:

``` yaml
Resources:
  FireLensPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Action:
          - logs:CreateLogStream
          - logs:CreateLogGroup
          - logs:DescribeLogStreams
          - logs:PutLogEvents
          Resource: "<resource ARN>"
Outputs:
  FireLensPolicyArn:
    Description: An addon ManagedPolicy gets used by the ECS task role
    Value: !Ref FireLensPolicy
```

!!!info
    Since the FireLens log driver can route your main container's logs to various destinations, the [`svc logs`](../commands/svc-logs.en.md) command can track them only when they are sent to the log group we create for your Copilot service in CloudWatch.

