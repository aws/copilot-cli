List of all available properties for a `'Environment'` manifest.  
To learn more about Copilot environments, see [Environments](../concepts/environments.en.md) concept page.

???+ note "Sample environment manifests"

    === "Basic"

        ```yaml
        name: prod
        type: Environment
        observability:
          container_insights: true
        ```

    === "Imported VPC"

        ```yaml
        name: imported
        type: Environment
        network:
          vpc:
            id: 'vpc-12345'
            subnets:
              public:
                - id: 'subnet-11111'
                - id: 'subnet-22222'
              private:
                - id: 'subnet-33333'
                - id: 'subnet-44444'
        ```

    === "Configured VPC"

        ```yaml
        name: qa
        type: Environment
        network:
          vpc:
            cidr: '10.0.0.0/16'
            subnets:
              public:
                - cidr: '10.0.0.0/24'
                  az: 'us-east-2a'
                - cidr: '10.0.1.0/24'
                  az: 'us-east-2b'
              private:
                - cidr: '10.0.3.0/24'
                  az: 'us-east-2a'
                - cidr: '10.0.4.0/24'
                  az: 'us-east-2b'
        ```

    === "With public certificates"

        ```yaml
        name: prod-pdx
        type: Environment
        http:
          public: # Apply an existing certificate to your public load balancer.
            certificates:
              - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/13245665-cv8f-adf3-j7gd-adf876af95
        ```

    === "Private"

        ```yaml
        name: onprem
        type: Environment
        network:
          vpc:
            id: 'vpc-12345'
            subnets:
              private:
                - id: 'subnet-11111'
                - id: 'subnet-22222'
                - id: 'subnet-33333'
                - id: 'subnet-44444'
        http:
          private: # Apply an existing certificate to your private load balancer.
            certificates:
              - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/13245665-cv8f-adf3-j7gd-adf876af95
            subnets: ['subnet-11111', 'subnet-22222']
        ```

    === "Content delivery network"

        ```yaml
        name: cloudfront
        type: Environment
        cdn: true
        http:
          public:
            security_groups:
             ingress:
               restrict_to:
                 cdn: true
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your environment.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Must be set to `'Environment'`.

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>  
The network section contains parameters for importing an existing VPC or configuring the Copilot-generated VPC.

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>  
The vpc section contains parameters to configure CIDR settings and subnets.

<span class="parent-field">network.vpc.</span><a id="network-vpc-id" href="#network-vpc-id" class="field">`id`</a> <span class="type">String</span>    
The ID of the VPC to import. This field is mutually exclusive with `cidr`.

<span class="parent-field">network.vpc.</span><a id="network-vpc-cidr" href="#network-vpc-cidr" class="field">`cidr`</a> <span class="type">String</span>    
An IPv4 CIDR block to associate with the Copilot-generated VPC. This field is mutually exclusive with `id`.

<span class="parent-field">network.vpc.</span><a id="network-vpc-subnets" href="#network-vpc-subnets" class="field">`subnets`</a> <span class="type">Map</span>    
Configure public and private subnets in a VPC.

For example, if you're importing an existing VPC:
```yaml
network:
  vpc:
    id: 'vpc-12345'
    public:
      - id: 'subnet-11111'
      - id: 'subnet-22222'
```
Alternatively, if you're configuring a Copilot-generated VPC:
```yaml
network:
  vpc:
    cidr: '10.0.0.0/16'
    subnets:
      public:
        - cidr: '10.0.0.0/24'
          az: 'us-east-2a'
        - cidr: '10.0.1.0/24'
          az: 'us-east-2b'
```

<span class="parent-field">network.vpc.subnets.</span><a id="network-vpc-subnets-public" href="#network-vpc-subnets-public" class="field">`public`</a> <span class="type">Array of Subnets</span>    
A list of public subnets configuration.

<span class="parent-field">network.vpc.subnets.</span><a id="network-vpc-subnets-private" href="#network-vpc-subnets-private" class="field">`private`</a> <span class="type">Array of Subnets</span>    
A list of private subnets configuration.

<span class="parent-field">network.vpc.subnets.<type\>.</span><a id="network-vpc-subnets-id" href="#network-vpc-subnets-id" class="field">`id`</a> <span class="type">String</span>    
The ID of the subnet to import. This field is mutually exclusive with `cidr` and `az`.

<span class="parent-field">network.vpc.subnets.<type\>.</span><a id="network-vpc-subnets-cidr" href="#network-vpc-subnets-cidr" class="field">`cidr`</a> <span class="type">String</span>    
An IPv4 CIDR block assigned to the subnet. This field is mutually exclusive with `id`.

<span class="parent-field">network.vpc.subnets.<type\>.</span><a id="network-vpc-subnets-az" href="#network-vpc-subnets-az" class="field">`az`</a> <span class="type">String</span>    
The Availability Zone name assigned to the subnet. The `az` field is optional, by default Availability Zones are assigned in alphabetical order.
This field is mutually exclusive with `id`.

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-group" href="#network-vpc-security-group" class="field">`security_group`</a> <span class="type">Map</span>  
Rules for the environment's security group.
```yaml
network:
  vpc:
    security_group:
      ingress:
        - ip_protocol: tcp
          ports: 80  
          cidr: 0.0.0.0/0
```
<span class="parent-field">network.vpc.security_group.</span><a id="network-vpc-security-group-ingress" href="#network-vpc-security-group-ingress" class="field">`ingress`</a> <span class="type">Array of Security Group Rules</span>    
A list of inbound security group rules.

<span class="parent-field">network.vpc.security-group.</span><a id="network-vpc-security-group-egress" href="#network-vpc-security-group-egress" class="field">`egress`</a> <span class="type">Array of Security Group Rules</span>    
A list of outbound security group rules.


<span class="parent-field">network.vpc.security_group.<type\>.</span><a id="network-vpc-security-group-ip-protocol" href="#network-vpc-security-group-ip-protocol" class="field">`ip_protocol`</a> <span class="type">String</span>    
The IP protocol name or number.

<span class="parent-field">network.vpc.security_group.<type\>.</span><a id="network-vpc-security-group-ports" href="#network-vpc-security-group-ports" class="field">`ports`</a> <span class="type">String or Integer</span>     
The port range or number for the security group rule.

```yaml
ports: 0-65535
```

or

```yaml
ports: 80
```

<span class="parent-field">network.vpc.security_group.<type\>.</span><a id="network-vpc-security-group-cidr" href="#network-vpc-security-group-cidr" class="field">`cidr`</a> <span class="type">String</span>   
The IPv4 address range, in CIDR format.


<div class="separator"></div>

<a id="cdn" href="#cdn" class="field">`cdn`</a> <span class="type">Boolean or Map</span>  
The cdn section contains parameters related to integrating your service with a CloudFront distribution. To enable the CloudFront distribution, specify `cdn: true`.

<span class="parent-field">cdn.</span><a id="cdn-certificate" href="#cdn-certificate" class="field">`certificate`</a> <span class="type">String</span>  
A certificate by which to enable HTTPS traffic on a CloudFront distribution.
CloudFront requires imported certificates to be in the `us-east-1` region. For example:

```yaml
cdn:
  certificate: "arn:aws:acm:us-east-1:1234567890:certificate/e5a6e114-b022-45b1-9339-38fbfd6db3e2"
```

<span class="parent-field">cdn.</span><a id="cdn-tls-termination" href="#cdn-tls-termination" class="field">`terminate_tls`</a> <span class="type">Boolean</span>  
Enable TLS termination for CloudFront.


<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
The http section contains parameters to configure the public load balancer shared by [Load Balanced Web Services](./lb-web-service.en.md)
and the internal load balancer shared by [Backend Services](./backend-service.en.md).

<span class="parent-field">http.</span><a id="http-public" href="#http-public" class="field">`public`</a> <span class="type">Map</span>  
Configuration for the public load balancer.

<span class="parent-field">http.public.</span><a id="http-public-certificates" href="#http-public-certificates" class="field">`certificates`</a> <span class="type">Array of Strings</span>  
List of [public AWS Certificate Manager certificate](https://docs.aws.amazon.com/acm/latest/userguide/gs-acm-request-public.html) ARNs.    
By attaching public certificates to your load balancer, you can associate your Load Balanced Web Services with a domain name and reach them with HTTPS.
See the [Developing/Domains](../developing/domain.en.md#use-domain-in-your-existing-validated-certificates) guide to learn more about how to redeploy services using [`http.alias`](./lb-web-service.en.md#http-alias).

<span class="parent-field">http.public.</span><a id="http-public-access-logs" href="#http-public-access-logs" class="field">`access_logs`</a> <span class="type">Boolean or Map</span>   
Enable [Elastic Load Balancing access logs](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html).   
If you specify `true`, Copilot will create an S3 bucket where the Public Load Balancer will store access logs.

```yaml
http:
  public:
    access_logs: true
```
You can customize the log prefix:
```yaml
http:
  public:
    access_logs:
      prefix: access-logs
```

It is also possible to use your own S3 bucket instead of letting Copilot creates one for you:
```yaml
http:
  public:
    access_logs:
      bucket_name: my-bucket
      prefix: access-logs
```

<span class="parent-field">http.public.access_logs.</span><a id="http-public-access-logs-bucket-name" href="#http-public-access-logs-bucket-name" class="field">`bucket_name`</a> <span class="type">String</span>   
The name of an existing S3 bucket in which to store the access logs.

<span class="parent-field">http.public.access_logs.</span><a id="http-public-access-logs-prefix" href="#http-public-access-logs-prefix" class="field">`prefix`</a> <span class="type">String</span>   
The prefix for the log objects.

<span class="parent-field">http.public.</span><a id="http-public-security-groups" href="#http-public-security-groups" class="field">`security_groups`</a> <span class="type">Map</span>    
Configure security groups to add to the public load balancer.

<span class="parent-field">http.public.security_groups.</span><a id="http-public-security-groups-ingress" href="#http-public-security-groups-ingress" class="field">`ingress`</a> <span class="type">Map</span>  
Ingress rules to allow for the public load balancer.  
```yaml
http:
  public:
    security_groups:
      ingress:
        restrict_to:
          cdn: true
```

<span class="parent-field">http.public.security_groups.ingress.</span><a id="http-public-security-groups-ingress-restrict-to" href="#http-public-security-groups-ingress-restrict-to" class="field">`restrict_to`</a> <span class="type">Map</span>  
Ingress rules to restrict the Public Load Balancer's traffic.

<span class="parent-field">http.public.security_groups.ingress.restrict_to.</span><a id="http-public-security-groups-ingress-restrict-to-cdn" href="#http-public-security-groups-ingress-restrict-to-cdn" class="field">`cdn`</a> <span class="type">Boolean</span>    
Restrict ingress traffic for the public load balancer to come from a CloudFront distribution.

<span class="parent-field">http.</span><a id="http-private" href="#http-private" class="field">`private`</a> <span class="type">Map</span>  
Configuration for the internal load balancer.

<span class="parent-field">http.private.</span><a id="http-private-certificates" href="#http-private-certificates" class="field">`certificates`</a> <span class="type">Array of Strings</span>  
List of [AWS Certificate Manager certificate](https://docs.aws.amazon.com/acm/latest/userguide/gs.html) ARNs.    
By attaching public or private certificates to your load balancer, you can associate your Backend Services with a domain name and reach them with HTTPS.
See the [Developing/Domains](../developing/domain.en.md#use-domain-in-your-existing-validated-certificates) guide to learn more about how to redeploy services using [`http.alias`](./backend-service.en.md#http-alias).

<span class="parent-field">http.private.</span><a id="http-private-subnets" href="#http-private-subnets" class="field">`subnets`</a> <span class="type">Array of Strings</span>   
The subnet IDs to place the internal load balancer in.

<span class="parent-field">http.private.</span><a id="http-private-security-groups" href="#http-private-security-groups" class="field">`security_groups`</a> <span class="type">Map</span>    
Configure security groups to add to the internal load balancer.

<span class="parent-field">http.private.security_groups</span><a id="http-private-security-groups-ingress" href="#http-private-security-groups-ingress" class="field">`ingress`</a> <span class="type">Map</span>  
Ingress rules to allow for the internal load balancer.  
```yaml
http:
  private:
    security_groups:
      ingress: # Enable incoming traffic within the VPC to the internal load balancer.
        from_vpc: true
```

<span class="parent-field">http.private.security_groups.ingress.</span><a id="http-private-security-groups-ingress-from-vpc" href="#http-private-security-groups-ingress-from-vpc" class="field">`from_vpc`</a> <span class="type">Boolean</span>    
Enable traffic from within the VPC to the internal load balancer.

<div class="separator"></div>

<a id="observability" href="#observability" class="field">`observability`</a> <span class="type">Map</span>  
The observability section lets you configure ways to collect data about the services and jobs deployed in your environment.

<span class="parent-field">observability.</span><a id="http-container-insights" href="#http-container-insights" class="field">`container_insights`</a> <span class="type">Bool</span>  
Whether to enable [CloudWatch container insights](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights.html) in your environment's ECS cluster.
