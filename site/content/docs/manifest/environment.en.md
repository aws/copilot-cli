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
            ingress:
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
    subnets:
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

<span class="parent-field">network.vpc.security_group.</span><a id="network-vpc-security-group-egress" href="#network-vpc-security-group-egress" class="field">`egress`</a> <span class="type">Array of Security Group Rules</span>    
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

<span class="parent-field">network.vpc.</span><a id="network-vpc-flowlogs" href="#network-vpc-flowlogs" class="field">`flow_logs`</a> <span class="type">Boolean or Map</span>   
If you specify 'true', Copilot will enable VPC flow logs to capture information about the IP traffic going in and out of the environment VPC.
The default value for VPC flow logs is 14 days (2 weeks).

```yaml
network:
  vpc:
    flow_logs: on
```
You can customize the number of days for retention:
```yaml
network:
  vpc:
    flow_logs:
      retention: 30
```
<span class="parent-field">network.vpc.flow_logs.</span><a id="network-vpc-flowlogs-retention" href="#network-vpc-flowlogs-retention" class="field">`retention`</a> <span class="type">String</span>
The number of days to retain the log events. See [this page](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-logs-loggroup.html#cfn-logs-loggroup-retentionindays) for all accepted values.

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

<span class="parent-field">cdn.</span><a id="cdn-static-assets" href="#cdn-static-assets" class="field">`static_assets`</a> <span class="type">Map</span>  
Optional. Configuration for static assets associated with CloudFront.

<span class="parent-field">cdn.static_assets.</span><a id="cdn-static-assets-alias" href="#cdn-static-assets-alias" class="field">`alias`</a> <span class="type">String</span>  
Additional HTTPS domain alias to use for static assets.

<span class="parent-field">cdn.static_assets.</span><a id="cdn-static-assets-location" href="#cdn-static-assets-location" class="field">`location`</a> <span class="type">String</span>  
DNS domain name of the S3 bucket (for example, `EXAMPLE-BUCKET.s3.us-west-2.amazonaws.com`).

<span class="parent-field">cdn.static_assets.</span><a id="cdn-static-assets-path" href="#cdn-static-assets-path" class="field">`path`</a> <span class="type">String</span>  
The path pattern (for example, `static/*`) that specifies which requests should be forwarded to the S3 bucket.

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

<span class="parent-field">http.public.</span><a id="http-public-sslpolicy" href="#http-public-sslpolicy" class="field">`ssl_policy`</a> <span class="type">String</span>   
Optional. Specify an SSL policy for the HTTPS listener of your Public Load Balancer, when applicable.

<span class="parent-field">http.public.</span><a id="http-public-ingress" href="#http-public-ingress" class="field">`ingress`</a> <span class="type">Map</span><span class="version">Modified in [v1.23.0](../../blogs/release-v123.en.md#move-misplaced-http-fields-in-environment-manifest-backward-compatible)</span>  
Ingress rules to restrict the Public Load Balancer's traffic.  

```yaml
http:
  public:
    ingress:
      cdn: true
```
???- note "<span class="faint"> "http.public.ingress" was previously "http.public.security_groups.ingress"</span>"  
    This field was `http.public.security_groups.ingress` until [v1.23.0](../../blogs/release-v123.en.md).
    This change cascaded to a child field [`cdn`](#http-public-ingress-cdn) (the only child field at the time), which was previously `http.public.security_groups.ingress.restrict_to.cdn`.
    For more, see [the blog post for v1.23.0](../../blogs/release-v123.en.md#move-misplaced-http-fields-in-environment-manifest-backward-compatible).

<span class="parent-field">http.public.ingress.</span><a id="http-public-ingress-cdn" href="#http-public-ingress-cdn" class="field">`cdn`</a> <span class="type">Boolean</span><span class="version">Modified in [v1.23.0](../../blogs/release-v123.en.md#move-misplaced-http-fields-in-environment-manifest-backward-compatible)</span>     
Restrict ingress traffic for the public load balancer to come from a CloudFront distribution.

<span class="parent-field">http.public.ingress.</span><a id="http-public-ingress-source-ips" href="#http-public-ingress-source-ips" class="field">`source_ips`</a> <span class="type">Array of Strings</span>    
Restrict public load balancer ingress traffic to source IPs.
```yaml
http:
  public:
    ingress:
      source_ips: ["192.0.2.0/24", "198.51.100.10/32"]  
```

<span class="parent-field">http.</span><a id="http-private" href="#http-private" class="field">`private`</a> <span class="type">Map</span>  
Configuration for the internal load balancer.

<span class="parent-field">http.private.</span><a id="http-private-certificates" href="#http-private-certificates" class="field">`certificates`</a> <span class="type">Array of Strings</span>  
List of [AWS Certificate Manager certificate](https://docs.aws.amazon.com/acm/latest/userguide/gs.html) ARNs.    
By attaching public or private certificates to your load balancer, you can associate your Backend Services with a domain name and reach them with HTTPS.
See the [Developing/Domains](../developing/domain.en.md#use-domain-in-your-existing-validated-certificates) guide to learn more about how to redeploy services using [`http.alias`](./backend-service.en.md#http-alias).

<span class="parent-field">http.private.</span><a id="http-private-subnets" href="#http-private-subnets" class="field">`subnets`</a> <span class="type">Array of Strings</span>
The subnet IDs to place the internal load balancer in.

<span class="parent-field">http.private.</span><a id="http-private-ingress" href="#http-private-ingress" class="field">`ingress`</a> <span class="type">Map</span><span class="version">Modified in [v1.23.0](../../blogs/release-v123.en.md#move-misplaced-http-fields-in-environment-manifest-backward-compatible)</span>  
Ingress rules to allow for the internal load balancer.  
```yaml
http:
  private:
    ingress:
      vpc: true  # Enable incoming traffic within the VPC to the internal load balancer.
```
???- note "<span class="faint"> "http.private.ingress" was previously "http.private.security_groups.ingress"</span>"  
    This field was `http.private.security_groups.ingress` until [v1.23.0](../../blogs/release-v123.en.md).
    This change cascaded to a child field [`vpc`](#http-private-ingress-vpc) (the only child field at the time),
    which was previously `http.private.security_groups.ingress.from_vpc`.
    For more, see [the blog post for v1.23.0](../../blogs/release-v123.en.md#move-misplaced-http-fields-in-environment-manifest-backward-compatible).

<span class="parent-field">http.private.ingress.</span><a id="http-private-ingress-vpc" href="#http-private-ingress-vpc" class="field">`vpc`</a> <span class="type">Boolean</span><span class="version">Modified in [v1.23.0](../../blogs/release-v123.en.md#move-misplaced-http-fields-in-environment-manifest-backward-compatible)</span>     
Enable traffic from within the VPC to the internal load balancer.

<span class="parent-field">http.private.</span><a id="http-private-sslpolicy" href="#http-private-sslpolicy" class="field">`ssl_policy`</a> <span class="type">String</span>   
Optional. Specify an SSL policy for the HTTPS listener of your Internal Load Balancer, when applicable.

<div class="separator"></div>

<a id="observability" href="#observability" class="field">`observability`</a> <span class="type">Map</span>  
The observability section lets you configure ways to collect data about the services and jobs deployed in your environment.

<span class="parent-field">observability.</span><a id="http-container-insights" href="#http-container-insights" class="field">`container_insights`</a> <span class="type">Bool</span>  
Whether to enable [CloudWatch container insights](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights.html) in your environment's ECS cluster.
