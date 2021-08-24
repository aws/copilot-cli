# Overrides
If there are fields that are not configurable in the [manifest](../manifest/overview.en.md), users can bypass some of them by applying override rules to the CloudFormation template Copilot generates out of the manifest.

## How to specify override rules?
For each override rule, users need to construct the **path** and **value** of the CloudFormation resource field they want to override.

``` yaml
<override section>:
  - path: <CFN field path>
    value: <value>
```

## Override Behaviors

- Copilot recursively creates `map` fields if they don't exist in the **path**. For example: if `B` and `C` don't exist, `A.B.C` will create `B` and `C`

- Use `-` as index to append a new member to a `list` field. The field will be initiated with this new member if it doesn't exist

- **value** must be scalar value like `bool`, `string`, or `int`, and the new value replaces the old value if exists

## Examples

### Task Definition Override
!!! Attention
    Users are not allowed to modify the following fields in the task definition.

    * Family
    * ContainerDefinitions[<index>].Name (name of any existing container)

Below is an example of adding [`Ulimits`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-containerdefinitions-ulimit.html) to the main container.

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].Ulimits[-].Name"
    value: "cpu"
  - path: "ContainerDefinitions[0].Ulimits[0].SoftLimit"
    value: 1024
  - path: "ContainerDefinitions[0].Ulimits[0].HardLimit"
    value: 2048
```

Below is exposing an extra UDP port for the main container.

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].PortMappings[-].ContainerPort"
    value: 2056
  - path: "ContainerDefinitions[0].PortMappings[1].Protocol"
    value: "udp"
```
