<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
The `network` section contains parameters for connecting to AWS resources in a VPC.

<span class="parent-field">network.</span><a id="network-connect" href="#network-connect" class="field">`connect`</a> <span class="type">Bool or Map</span>    
Enable [Service Connect](../developing/svc-to-svc-communication.en.md#service-connect) for your service, which makes the traffic between services load balanced and more resilient. Defaults to `false`.

When using it as a map, you can specify which alias to use for this service. Note that the alias must be unique within the environment.

<span class="parent-field">network.connect.</span><a id="network-connect-alias" href="#network-connect-alias" class="field">`alias`</a> <span class="type">String</span>  
A custom DNS name for this service exposed to Service Connect. Defaults to the service name.

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>    
Subnets and security groups attached to your tasks.

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String or Map</span>  
When using it as a string, the value must be one of `'public'` or `'private'`. Defaults to launching your tasks in public subnets.

!!! info
    If you launch tasks in `'private'` subnets and use a Copilot-generated VPC, Copilot will automatically add NAT Gateways to your environment for internet connectivity. (See [pricing](https://aws.amazon.com/vpc/pricing/).) Alternatively, when running `copilot env init`, you can import an existing VPC with NAT Gateways, or one with VPC endpoints for isolated workloads. See our [custom environment resources](../developing/custom-environment-resources.en.md) page for more.

When using it as a map, you can specify in which subnets Copilot should launch ECS tasks. For example:

```yaml
network:
  vpc:
    placement:
      subnets: ["SubnetID1", "SubnetID2"]
```

<span class="parent-field">network.vpc.placement.</span><a id="network-vpc-placement-subnets" href="#network-vpc-placement-subnets" class="field">`subnets`</a> <span class="type">Array of Strings or Map</span>  
As a list of strings, the subnet IDs where Copilot should launch ECS tasks.

As a map, the name-value pairs by which to filter your subnets. Note that the filters are joined with an `AND`, and the values for each filter are joined by an `OR`. For example, both subnets with tag set `org: bi` and `type: public`, and subnets with tag set `org: bi` and `type: private` will be matched by

```yaml
network:
  vpc:
    placement:
      subnets:
        from_tags:
          org: bi
          type:
            - public
            - private
```

<span class="parent-field">network.vpc.placement.subnets</span><a id="network-vpc-placement-subnets-from-tags" href="#network-vpc-placement-subnets-from-tags" class="field">`from_tags`</a> <span class="type">Map of String and String or Array of Strings</span>  
Tag sets by which to filter subnets where Copilot should launch ECS tasks.

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings or Map</span>  
Additional security group IDs associated with your tasks.
```yaml
network:
  vpc:
    security_groups: [sg-0001, sg-0002]
```
Copilot includes a security group so containers within your environment can communicate with each other. To disable
the default security group, you can specify the `Map` form:
```yaml
network:
  vpc:
    security_groups:
      deny_default: true
      groups: [sg-0001, sg-0002]
```

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-from-cfn" href="#network-vpc-security-groups-from-cfn" class="field">`from_cfn`</a> <span class="type">String</span>  
The name of a [CloudFormation stack export](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html).

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-deny-default" href="#network-vpc-security-groups-deny-default" class="field">`deny_default`</a> <span class="type">Boolean</span>  
Disable the default security group that allows ingress from all services in your environment.

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-groups" href="#network-vpc-security-groups-groups" class="field">`groups`</a> <span class="type">Array of Strings</span>    
Additional security group IDs associated with your tasks.

<span class="parent-field">network.vpc.security_groups.groups</span><a id="network-vpc-security-groups-groups-from-cfn" href="#network-vpc-security-groups-groups-from-cfn" class="field">`from_cfn`</a> <span class="type">String</span>  
The name of a [CloudFormation stack export](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html). 
