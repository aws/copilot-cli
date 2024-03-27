<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
The `network` section contains parameters for connecting to AWS resources in a VPC.

<span class="parent-field">network.</span><a id="network-connect" href="#network-connect" class="field">`connect`</a> <span class="type">Bool or Map</span>    
Enable [Service Connect](../developing/svc-to-svc-communication.en.md#service-connect) for your service, which makes the traffic between services load balanced and more resilient. Defaults to `false`.

When using it as a map, you can specify which alias to use for this service. Note that the alias must be unique within the environment.

<span class="parent-field">network.connect.</span><a id="network-connect-alias" href="#network-connect-alias" class="field">`alias`</a> <span class="type">String</span>  
A custom DNS name for this service exposed to Service Connect. Defaults to the service name.

{% include 'network-vpc.en.md' %}