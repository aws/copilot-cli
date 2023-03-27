---
title: 'AWS Copilot v1.27: Extend Copilot templates, additional routing rule supports, preview differences, and sidecar improvements!'
twitter_title: 'AWS Copilot v1.27'
image: 'https://user-images.githubusercontent.com/879348/227655119-e42c6b8b-ff0e-4abe-ad90-89b44813fbd5.png'
image_alt: 'CDK overrides and diff'
image_width: '2056'
image_height: '1096'
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

AWS Copilot enables builders to quickly get started with containerized applications through the `copilot init` command and following prompts.
Developers can then grow their applications by editing and deploying [manifest](../docs/manifest/overview.en.md) infrastructure-as-code files.
And now with v1.27, developers can extend any Copilot-generated CloudFormation template with the `copilot [noun] override` command, enabling them to fully customize their infrastructure.

### AWS Cloud Development Kit (CDK) Overrides

You can use the CDK to extend your CloudFormation templates when you need the expressive power and safety of a
programming language. After running the `copilot [noun] override` command, Copilot will generate a CDK application
under the `copilot/[name]/override` directory:

```console
.
├── bin/
│   └── override.ts
├── .gitignore
├── cdk.json
├── package.json
├── README.md
├── stack.ts
└── tsconfig.json
```

You can get started with adding, removing, or replacing properties by editing the `stack.ts` file.

??? note "View sample `stack.ts`"

    ```typescript
    import * as cdk from 'aws-cdk-lib';
    import * as path from 'path';
    import { aws_elasticloadbalancingv2 as elbv2 } from 'aws-cdk-lib';
    import { aws_ec2 as ec2 } from 'aws-cdk-lib';
    
    interface TransformedStackProps extends cdk.StackProps {
        readonly appName: string;
        readonly envName: string;
    }
    
    export class TransformedStack extends cdk.Stack {
        public readonly template: cdk.cloudformation_include.CfnInclude;
        public readonly appName: string;
        public readonly envName: string;
    
        constructor (scope: cdk.App, id: string, props: TransformedStackProps) {
            super(scope, id, props);
            this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
                templateFile: path.join('.build', 'in.yml'),
            });
            this.appName = props.appName;
            this.envName = props.envName;
            this.transformPublicNetworkLoadBalancer();
        }
    
        /**
         * transformPublicNetworkLoadBalancer removes the "Subnets" properties from the NLB,
         * and adds a SubnetMappings with predefined elastic IP addresses.
         */
        transformPublicNetworkLoadBalancer() {
            const elasticIPs = [new ec2.CfnEIP(this, 'ElasticIP1'), new ec2.CfnEIP(this, 'ElasticIP2')];
            const publicSubnets = cdk.Fn.importValue(`${this.appName}-${this.envName}-PublicSubnets`);
    
            // Apply the override.
            const nlb = this.template.getResource("PublicNetworkLoadBalancer") as elbv2.CfnLoadBalancer;
            nlb.addDeletionOverride('Properties.Subnets');
            nlb.subnetMappings = [{
                allocationId: elasticIPs[0].attrAllocationId,
                subnetId: cdk.Fn.select(0, cdk.Fn.split(",", publicSubnets)),
            }, {
                allocationId: elasticIPs[1].attrAllocationId,
                subnetId: cdk.Fn.select(1, cdk.Fn.split(",", publicSubnets)),
            }]
        }
    }
    ```

To learn more about extending with the CDK, checkout the [guide](../docs/developing/overrides/cdk.md).

### YAML Patch Overrides

You can use YAML Patch overrides for a more lightweight experience when 1) you do not want to have a dependency
on any other tooling and framework, or 2) you have to write only a handful modifications. 
After running the `copilot [noun] override` command, Copilot will generate a sample `cfn.patches.yml` file
under the `copilot/[name]/override` directory:

```console
.
├── cfn.patches.yml
└── README.md
```

You can get started with adding, removing, or replacing properties by editing the `cfn.patches.yaml` file.

??? note "View sample `cfn.patches.yml`"

    ```yaml
    - op: add
      path: /Mappings
      value:
        ContainerSettings:
          test: { Cpu: 256, Mem: 512 }
          prod: { Cpu: 1024, Mem: 1024}
    - op: remove
      path: /Resources/TaskRole
    - op: replace
      path: /Resources/TaskDefinition/Properties/ContainerDefinitions/1/Essential
      value: false
    - op: add
      path: /Resources/Service/Properties/ServiceConnectConfiguration/Services/0/ClientAliases/-
      value:
        Port: !Ref TargetPort
        DnsName: yamlpatchiscool
    ```

To learn more about extending with YAML patches, check out the [guide](../docs/developing/overrides/yamlpatch.md).

## Preview AWS CloudFormation template changes

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

### Build and push container images for sidecar containers

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