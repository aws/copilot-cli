<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
The `network` section contains parameters for connecting to AWS resources in a VPC.

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>    
Subnets and security groups attached to your tasks.

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
Must be one of `'public'` or `'private'`. Defaults to launching your tasks in public subnets.

!!! info
    If you launch tasks in `'private'` subnets and use a Copilot-generated VPC, Copilot will automatically add NAT Gateways to your environment for internet connectivity. (See [pricing](https://aws.amazon.com/vpc/pricing/).) Alternatively, when running `copilot env init`, you can import an existing VPC with NAT Gateways, or one with VPC endpoints for isolated workloads. See our [custom environment resources](../developing/custom-environment-resources.en.md) page for more.

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>  
Additional security group IDs associated with your tasks. Copilot always includes a security group so containers within your environment
can communicate with each other.
