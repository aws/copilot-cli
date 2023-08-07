# CDK Overrides

{% include 'overrides-intro.md' %}

## When should I use CDK overrides over YAML patch?

Both options are a "break the glass" mechanism to access and configure functionality that is not surfaced by Copilot [manifests](../../manifest/overview.en.md).

We recommend using the AWS Cloud Development Kit (CDK) overrides over [YAML patches](./yamlpatch.md) if you'd like to leverage
the expressive power of a programming language. The CDK allows you to make safe and powerful modifications to your CloudFormation template.

## How to get started

You can extend your CloudFormation template with the CDK by running the `copilot [noun] override` command.
For example, you can run `copilot svc override` to update the template of a Load Balanced Web Service.

The command will generate a new CDK application under the `copilot/[name]/override` directory with the following structure:
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

You can get started by editing the `stack.ts` file. For example, if you decided to override the ECS service properties
with `copilot svc override`, the following `stack.ts` file will be generated for you to modify:

```typescript
import * as cdk from 'aws-cdk-lib';
import { aws_ecs as ecs } from 'aws-cdk-lib';

export class TransformedStack extends cdk.Stack {
    constructor (scope: cdk.App, id: string, props?: cdk.StackProps) {
        super(scope, id, props);
         this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
            templateFile: path.join('.build', 'in.yaml'),
        });
        this.appName = template.getParameter('AppName').valueAsString;
        this.envName = template.getParameter('EnvName').valueAsString;

        this.transformService();
    }
 
    // TODO: implement me.
    transformService() {
      const service = this.template.getResource("Service") as ecs.CfnService;
      throw new error("not implemented");
    }
}
```

## How does it work?

As can be seen in the above `stack.ts` file, Copilot will use the [cloudformation_include module](https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.cloudformation_include-readme.html) 
provided by the CDK to help author transformations. This library is the CDK’s recommendation from their 
["Import or migrate an existing AWS CloudFormation template"](https://docs.aws.amazon.com/cdk/v2/guide/use_cfn_template.html) guide. It enables accessing the resources not surfaced by the Copilot manifest as 
[L1 constructs](https://docs.aws.amazon.com/cdk/v2/guide/constructs.html).  
The `CfnInclude` object is initialized from the hidden `.build/in.yaml` CloudFormation template. 
This is how Copilot and the CDK communicate. 
Copilot writes the manifest-generated CloudFormation template under the `.build/` directory, 
which then gets parsed by the `cloudformation_include` library into a CDK construct.

Every time you run `copilot [noun] package` or `copilot [noun] deploy`, Copilot will first generate the CloudFormation template 
from the manifest file, and then pass it down to your CDK application to override properties.

We highly recommend using the `--diff` flag with the `package` or `deploy` command to first visualize your CDK changes before a deployment.

## Examples

The following example modifies the [`nlb`](../../manifest/lb-web-service.en.md#nlb) resource of a Load Balanced Web Service to
assign Elastic IP addresses to the Network Load Balancer.

In this example, you can view how to:

- Delete a resource property.
- Create new resources.
- Modify a property of an existing resource.

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

The following example showcases how you can add a property for only a particular environment, such as a production environment:

??? note "View sample `stack.ts`"

    ```typescript
    import * as cdk from 'aws-cdk-lib';
    import * as path from 'path';
    import { aws_iam as iam } from 'aws-cdk-lib';
    
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
            this.transformEnvironmentManagerRole();
        }
        
        transformEnvironmentManagerRole() {
            const environmentManagerRole = this.template.getResource("EnvironmentManagerRole") as iam.CfnRole;
            if (this.envName === "prod") {
                let assumeRolePolicy = environmentManagerRole.assumeRolePolicyDocument
                let statements = assumeRolePolicy.Statement
                statements.push({
                     "Effect": "Allow",
                     "Principal": { "Service": "ec2.amazonaws.com" },
                     "Action": "sts:AssumeRole"
                })
            }
        }
    }
    ```

The following example showcases how you can delete a resource, the Copilot-created default log group that holds service logs.

??? note "View sample `stack.ts`"

    ```typescript
    import * as cdk from 'aws-cdk-lib';
    import * as path from 'path';

    interface TransformedStackProps extends cdk.StackProps {
        readonly appName: string;
        readonly envName: string;
    }

    export class TransformedStack extends cdk.Stack {
        public readonly template: cdk.cloudformation_include.CfnInclude;
        public readonly appName: string;
        public readonly envName: string;

        constructor(scope: cdk.App, id: string, props: TransformedStackProps) {
            super(scope, id, props);
            this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
            templateFile: path.join('.build', 'in.yml'),
            });
            this.appName = props.appName;
            this.envName = props.envName;
            // Deletes the default log group resource.
            this.template.node.tryRemoveChild("LogGroup")
        }
    }
    ```
