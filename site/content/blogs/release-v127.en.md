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
  [See detailed section](#enable-multiple-listeners-and-listener-rules-for-load-balancers).
- **Preview CloudFormation template changes**: You can now run `copilot [noun] package` or `copilot [noun] deploy` commmands with the `--diff` flag to show differences
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

## Enable multiple listeners and listener rules for Load Balancers
You can now expose multiple ports through your Load Balancers.
### Add multiple host-based or path-based listener rules to your Application Load Balancer
To expose multiple ports through Application Load Balancer, we will configure additional listener rules through a new `http` field called `additional_rules`. 
It is as easy as configuring your `http` field. Let's learn through an example. 

Say we want to expand the basic manifest such that it opens up port 8081 on the main service container, and 8082 on the sidecar container, in addition to the existing `image.port` 8080.
```yaml
# Example 1
name: 'frontend'
type: 'Load Balanced Web Service'
 
image:
  build: './frontend/Dockerfile'
  port: 8080
  
http:
  path: '/'
  additional_rules:             # The new field "additional_rules",
    - target_port: 8081        # Optional. Defaults to the `image.port`.
      path: 'customerdb'
    - target_port: 8082
      target_container: nginx   # Optional. Defaults to the main container. 
      path: 'admin'
    - target_port: 80
      path: 'superAdmin'

sidecars:
  nginx:
    port: 80
    image: public.ecr.aws/nginx:latest
```
With this manifest, requests to “/” will still be routed to the main container’s port 8080. Requests to "/customerdb" will be route to the main container’s 8081, 
, "/admin" to “nginx”‘s port 8082 and "/superAdmin" to "nginx"'s port 80. We should note that the third listener rule just defined 'target_port: 80' 
and Copilot was able to intelligently route traffic from the '/superAdmin' to the sidecar container named nginx.

It is also possible to configure the container port that handles the requests to “/” via our new field under `http` called [`target_port`]()
### Add multiple port and protocol listeners to your Network Load Balancers
To expose multiple ports through Network Load Balancer, we will configure additional listener through a new `nlb` field called `additional_listeners`.
It is as easy as configuring your `nlb` field. Let's learn through an example.

```yaml
name: 'frontend'
type: 'Load Balanced Web Service'

image:
  build: Dockerfile

http: false
nlb:
  port: 8080/tcp
  additional_listeners:
    - port: 8081/tcp
    - port: 8082/tcp
      target_port: 8085               # Optional. Default is set to nlb port of the specific listener 
      target_container: nginx         # Optional. Default is set to the main container

sidecars:
  nginx:
    port: 80
    image: public.ecr.aws/nginx:latest
```
With this manifest, requests to NLB port 8080/tcp will now be routed to the main container’s port 8080. 
Requests to another NLB port 8081 will be routed to the port 8081 of the main service container. 
We need to notice here that the default value of the target_port will be the same as that of the corresponding NLB port. 
The requests to NLB port 8082 will be routed to port 8085 of the sidecar container named nginx.

## Preview AWS CloudFormation template changes

## Sidecar improvements

### Build and push container images for sidecar containers

### Upload local environment files for sidecar containers
