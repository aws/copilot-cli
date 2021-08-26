# Task Definition Overrides
Copilot generates CloudFormation templates using configuration specified in the [manifest](../manifest/overview.en.md). However, there are fields that are not configurable in the manifest. For example, You might want to configure the [`Ulimits`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-containerdefinitions.html#cfn-ecs-taskdefinition-containerdefinition-ulimits) for your workload container, but it is not exposed in our manifest.

You can configure [ECS Task Definition settings](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html) by specifying override rules, which will be applied to the CloudFormation template that Copilot generates out of the manifest.

## How to specify override rules?
For each override rule, you need to specify a **path** of the CloudFormation resource field you want to override, and a **value** of that field.

``` yaml
taskdef_overrides:
  - path: <ECS Task Definition field path>
    value: <value>
```

## Override Behaviors

- Use `-` as index to append a new member to a `list` field

- When applying an override rule, Copilot inserts or updates the fields along the path. More specifically, Copilot recursively inserts fields if they don't exist in the CloudFormation template. For example: if a rule has the path `A.B[-].C` (`B` and `C` don't exist), Copilot will insert the field `B` and `C`. A concrete example can be found [below](#add-ulimits-to-the-main-container).

!!! Attention
    The following fields in the task definition are not allowed to be modified.

    * [Family](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html#cfn-ecs-taskdefinition-family)
    * [ContainerDefinitions[<index>].Name](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-containerdefinitions.html#cfn-ecs-taskdefinition-containerdefinition-name)

## Examples

### Add `Ulimits` to the main container

``` yaml
taskdef_overrides:
  - path: ContainerDefinitions[0].Ulimits[-]
    value:
      Name: "cpu"
      SoftLimit: 1024
      HardLimit: 2048
```

### Expose an extra UDP port

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].PortMappings[-].ContainerPort"
    value: 2056
  // PortMappings[1] gets the port mapping added by the previous rule, since by default Copilot creates a port mapping.
  - path: "ContainerDefinitions[0].PortMappings[1].Protocol"
    value: "udp"
```

### Give read-only access to the root file system

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].ReadonlyRootFilesystem"
    value: true
```
