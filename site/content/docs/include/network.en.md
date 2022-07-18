<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
The `network` section contains parameters for connecting to AWS resources in a VPC.

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

<span class="parent-field">network.vpc.placement.</span><a id="network-vpc-placement-subnets" href="#network-vpc-placement-subnets" class="field">`subnets`</a> <span class="type">Array of Strings</span>  
A list of subnet IDs where Copilot launches ECS tasks.

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings or Map</span>  
Additional security group IDs associated with your tasks. 
```yaml
network:
  vpc:
    security_groups: [sg-0001, sg-0002]
```
Copilot includes a security group so containers within your environment can communicate with each other. To disable 
the default security group, you can specify the Map form:
```yaml
network:
  vpc:
    security_groups:
      deny_default: true
      groups: [sg-0001, sg-0002]
```

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-deny-default" href="#network-vpc-security-groups-deny-default" class="field">`deny_default`</a> <span class="type">Boolean</span>  
Disable the default security group that allows ingress from all services in your environment.

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-groups" href="#network-vpc-security-groups-groups" class="field">`groups`</a> <span class="type">Array of Strings</span>    
Additional security group IDs associated with your tasks.

