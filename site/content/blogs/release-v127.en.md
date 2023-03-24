---
title: 'AWS Copilot v1.27: Extend Copilot templates, additional routing rule supports, preview differences, and sidecar improvements!'
twitter_title: 'AWS Copilot v1.27'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.27: Extend Copilot templates, additional routing rule supports, preview differences, and sidecar improvements!
##### Posted On: Mar 28, 2023

The AWS Copilot core team is announcing the Copilot v1.27 release 🚀.  
Our public [сommunity сhat](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) is growing and has over 400 people online and over 2.7k stars on [GitHub](http://github.com/aws/copilot-cli/).
Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.27 is a big release with several new features and improvements:

- **Extend Copilot templates**: You can now customize any properties in Copilot-generated AWS CloudFormation templates 
with the AWS Cloud Development Kit (CDK) or YAML Patch overrides. [See detailed section](#extend-copilot-generated-aws-cloudformation-templates).
- **Enable multiple listeners and listener rules**: You can define multiple host-based or path listener rules for [application load balancers](../docs/manifest/lb-web-service.en.md#http)
or multiple listeners on different ports and protocols for [network load balancers](../docs/manifest/lb-web-service.en.md#nlb).  
  [See detailed section](#enable-multiple-listeners-and-routing-rules-for-load-balancers).
- **Preview CloudFormation template changes**: You can now run `copilot [noun] package` or `copilot [noun] deploy` commands with the `--diff` flag to show differences
  between the last deployed CloudFormation template and local changes. [See detailed section](#preview-aws-cloudformation-template-changes).
- **Build and push container images for sidecars**: Add support for `image.build` to build and push sidecar containers from local Dockerfiles. [See detailed section](#build-and-push-container-images-for-sidecar-containers).
- **Environment file support for sidecars**: Add support for `env_file` to push a local `.env` file for sidecar containers. [See detailed section](#upload-local-environment-files-for-sidecar-containers).

??? note "What’s AWS Copilot?"

    The AWS Copilot CLI is a tool for developers to build, release, and operate production-ready containerized applications on AWS.
    From getting started to releasing in production, Copilot can help manage the entire lifecycle of your application development.
    At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code.
    Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of microservice architectures,
    enabling you to focus on developing your application instead of writing deployment scripts.

    See the [Overview](../docs/concepts/overview.en.md) section for a more detailed introduction to AWS Copilot.

## Extend Copilot-generated AWS CloudFormation templates

#### Preview AWS CloudFormation template changes

##### `copilot [noun] package --diff`

You can now run `copilot [noun] package --diff` to see the diff between your local changes and the latest deployed template. 
The program will exit after it prints the diff. 

!!! info "The exit codes when using `copilot [noun] package --diff`"
    0 = no diffs found  
    1 = diffs found  
    2 = error producing diffs


```console
$ copilot env deploy --diff
~ Resources:
    ~ Cluster:
        ~ Properties:
            ~ ClusterSettings:
                ~ - (changed item)
                  ~ Value: enabled -> disabled
```

If the diff looks good to you, you can run `copilot [noun] package` again to write the template file and parameter file
to your designated directory.


##### `copilot [noun] deploy --diff`

Similar to `copilot [noun] package --diff`, you can run `copilot [noun] deploy --diff` to see the same diff. 
However, instead of exiting after it print the diff, Copilot will follow up with a question: `Continue with the deployment? [y/N]`.

```console
$ copilot job deploy --diff
~ Resources:
    ~ TaskDefinition:
        ~ Properties:
            ~ ContainerDefinitions:
                ~ - (changed item)
                  ~ Environment:
                      (4 unchanged items)
                      + - Name: LOG_LEVEL
                      +   Value: "info"

Continue with the deployment? (y/N)
```

If the diff looks good to you, enter "y" to deploy. Otherwise, enter "N" to make adjustments as needed!

## Enable multiple listeners and routing rules for Load Balancers

### Add multiple host-based or path-based routing rules to your Application Load Balancers

### Add multiple port and protocol listeners to your Network Load Balancers

## Sidecar improvements

You can now build and push container images for sidecar containers just like you can for your main container.
Additionally, you can now specify the paths to local environment files for sidecar containers.

### Build and push container images for sidecar containers

Copilot now allows users to build sidecar container images natively from Dockerfiles and push them to ECR.
In order to take advantage of this feature, users can modify their workload manifests in several ways.

The first option is to simply specify the path to the Dockerfile as a string.

```yaml
sidecars:
  nginx:
    image:
      build: path/to/dockerfile
```

Alternatively, you can specify `build` as a map, which allows for more advanced customization. 
This includes specifying the Dockerfile path, context directory, target build stage, cache from images, and build arguments.

```yaml
sidecars:
  nginx:
    image:
      build:
        dockerfile: path/to/dockerfile
        context: context/dir
        target: build-stage
        cache_from:
          - image: tag
        args: value
```

Another option is to specify an existing image URI instead of building from a Dockerfile.

```yaml
sidecars:
  nginx:
    image: 123457839156.dkr.ecr.us-west-2.amazonaws.com/demo/front:nginx-latest
```

Or you can provide the image URI using the location field.
```yaml
sidecars:
  nginx:
    image:
      location:  123457839156.dkr.ecr.us-west-2.amazonaws.com/demo/front:nginx-latest
```

### Upload local environment files for sidecar containers
You can now specify an environment file to upload to any sidecar container in your task.
Previously, you could only specify an environment file for your main task container: 

```yaml
# in copilot/{service name}/manifest.yml
env_file: log.env
```

Now, you can do the same in a sidecar definition:
```yaml
sidecars:
  nginx:
    image: nginx:latest
    env_file: ./nginx.env
    port: 8080
```

It also works with the managed `logging` sidecar:

```yaml
logging:
  retention: 1
  destination:
    Name: cloudwatch
    region: us-west-2
    log_group_name: /copilot/logs/
    log_stream_prefix: copilot/
  env_file: ./logging.env
```

If you specify the same file more than once in different sidecars, Copilot will only upload the file to S3 once.