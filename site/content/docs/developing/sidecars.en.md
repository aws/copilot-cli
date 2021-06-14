# Sidecars
Sidecars are additional containers that run along side the main container. They are usually used to perform peripheral tasks such as logging, configuration, or proxying requests.

AWS also provides some plugin options that can be seamlessly incorporated with your ECS service, including but not limited to [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html), [AWS X-Ray](https://aws.amazon.com/xray/), and [AWS App Mesh](https://aws.amazon.com/app-mesh/).

If you have defined an EFS volume for your main container through the [`storage` field](../developing/storage.en.md) in the manifest, you can also mount that volume in any sidecar containers you have defined.

## How to add sidecars with Copilot?
There are two ways of adding sidecars using the Copilot manifest: by specifying [general sidecars](#general-sidecars) or by using [sidecar patterns](#sidecar-patterns).

!!! Attention
    Sidecars are not supported for Request-Driven Web Services

### General sidecars
You'll need to provide the URL for the sidecar image. Optionally, you can specify the port you'd like to expose and the credential parameter for [private registry](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html).

``` yaml
sidecars:
  <sidecar name>:
    # Port of the container to expose. (Optional)
    port: <port number>
    # Image URL for the sidecar container. (Required)
    image: <image url>
    # ARN of the secret containing the private repository credentials. (Optional)
    credentialsParameter: <credential>
    # Environment variables for the sidecar container.
    variables: <env var>
    # Secrets to expose to the sidecar container.
    secrets: <secret>
    # Mount paths for EFS volumes specified at the service level. (Optional)
    mount_points:
      - # Source volume to mount in this sidecar. (Required)
        source_volume: <named volume>
        # The path inside the sidecar container at which to mount the volume. (Required)
        path: <path>
        # Whether to allow the sidecar read-only access to the volume. (Default true)
        read_only: <bool>
    # Optional Docker labels to apply to this container.
    labels:
      {label key} : <label value>
    # Optional container dependencies to apply to this container.
    depends_on:
      {container name}: <condition>

```

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
  targetContainer: 'nginx'

cpu: 256
memory: 512
count: 1

sidecars:
  nginx:
    port: 80
    image: 1234567890.dkr.ecr.us-west-2.amazonaws.com/reverse-proxy:revision_1
    variables:
      NGINX_PORT: 80
```

Below is a fragment of a manifest including an EFS volume in both the service and sidecar container.

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

### Sidecar patterns
Sidecar patterns are predefined Copilot sidecar configurations. For now, the only supported pattern is FireLens, but we'll add more in the future!

``` yaml
# In the manifest.
logging:
  # The Fluent Bit image. (Optional, we'll use "amazon/aws-for-fluent-bit:latest" by default)
  image: <image URL>
  # The configuration options to send to the FireLens log driver. (Optional)
  destination:
    <config key>: <config value>
  # Whether to include ECS metadata in logs. (Optional, default to true)
  enableMetadata: <true|false>
  # Secret to pass to the log configuration. (Optional)
  secretOptions:
    <key>: <value>
  # The full config file path in your custom Fluent Bit image.
  configFilePath: <config file path>
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

You might need to add necessary permissions to the task role so that FireLens can forward your data. You can add permissions by specifying them in your [addons](../developing/additional-aws-resources.en.md). For example:

``` yaml
Resources:
  FireLensPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: 2012-10-17
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

!!!info
    ** We're going to make this easier and more powerful!** Currently, we only support using remote images for sidecars, which means users need to build and push their local sidecar images. But we are planning to support using local images or Dockerfiles. Additionally, FireLens will be able to route logs for the other sidecars (not just the main container).
