# Welcome to overriding your Copilot generated CloudFormation template with the CDK

This is a CDK project with TypeScript to extend the CloudFormation template that gets 
deployed with AWS Copilot.

The files of special importance are:
- `package.json` file holds the version of the CDK Toolkit and Library that Copilot will use to apply the overrides.
- `stack.ts` file holds the transformations to apply to the CloudFormation template.
- `bin/override.ts` file holds the entrypoint to the CDK application.

## Troubleshooting

* `copilot [noun] package` preview the transformed template by writing to stdout.
* `copilot [noun] package --diff` show the difference against the template deployed in your environment.

## Under the hood
The `stack.ts` file follows the [import or migrate an existing AWS CloudFormation template guide](https://docs.aws.amazon.com/cdk/v2/guide/use_cfn_template.html) by using the `cloudformation-include.CfnInclude` construct
from the CDK to transform the Copilot-generated CloudFormation template into AWS CDK L1 constructs.  
By writing `transform()` methods in stack, you can access and modify properties of the resources.

The CDK and Copilot communicate when running `copilot [noun] package`:
1. Copilot copies the template generated from your `manifest.yml` under `.build/in.yml`.
2. Copilot then runs `cdk synth` from your `overrides/` directory and uses its output to deploy to CloudFormation.

## Additional Guides

To learn more about Copilot CDK overrides and view examples, check out [the documentation](https://aws.github.io/copilot-cli/docs/developing/overrides/cdk/).  
To learn how to edit L1 CDK constructs, check out [the CDK documentation](https://docs.aws.amazon.com/cdk/v2/guide/cfn_layer.html).