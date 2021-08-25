# Task Definition Overrides
If there are fields that are not configurable in the [manifest](../manifest/overview.en.md), users can bypass some of the [ECS Task Definition setting](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html) by applying override rules to the CloudFormation template Copilot generates out of the manifest.

## How to specify override rules?
For each override rule, users need to construct the **path** and **value** of the CloudFormation resource field they want to override.

``` yaml
taskdef_overrides:
  - path: <ECS Task Definition field path>
    value: <value>
```

## Override Behaviors

- Use `-` as index to append a new member to a `list` field

- When applying the override rule, Copilot inserts or updates the fields along the path. More specifically, Copilot recursively inserts fields if they don't exist in the **path**. For example: if `B` and `C` don't exist, `A.B[-].C` will create `B` and `C`

!!! Attention
    Users are not allowed to modify the following fields in the task definition.

    * [Family](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html#cfn-ecs-taskdefinition-family)
    * [ContainerDefinitions[<index>].Name](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-containerdefinitions.html#cfn-ecs-taskdefinition-containerdefinition-name)

## Examples

**Add [`Ulimits`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-containerdefinitions-ulimit.html) to the main container**

``` yaml
taskdef_overrides:
  - path: ContainerDefinitions[0].Ulimits[-]
    value:
      Name: "cpu"
      SoftLimit: 1024
      HardLimit: 2048
```

**Expose an extra UDP port**

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].PortMappings[-].ContainerPort"
    value: 2056
  // PortMappings[1] gets the port mapping added by the previous rule, since by default Copilot creates a port mapping.
  - path: "ContainerDefinitions[0].PortMappings[1].Protocol"
    value: "udp"
```

**Give read-only access to the root file system**

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].ReadonlyRootFilesystem"
    value: true
```
