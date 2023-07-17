!!! Attention

    :warning: Overriding CloudFormation templates is an advanced feature which can cause your stacks to not deploy successfully. 
    Please use with caution!

Copilot generates CloudFormation templates using configuration specified in the [manifest](../../manifest/overview.md).
However, not all CloudFormation properties are configurable in the manifest.
For example, you might want to configure the [`Ulimits`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-ulimit.html)
for your workload container, but the property is not exposed in manifests.

Overrides with `yamlpatch` or `cdk` allow you to add, delete, or replace _any_ property or resource in a CloudFormation template.