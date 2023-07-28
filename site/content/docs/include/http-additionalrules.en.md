<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-path" href="#http-additional-rules-path" class="field">`path`</a> <span class="type">String</span>  
    Requests to this path will be forwarded to your service. Each listener rule should listen on a unique path.
    
{% include 'http-additionalrules-healthcheck.en.md' %}
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-deregistration-delay" href="#http-additional-rules-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>  
    The amount of time to wait for targets to drain connections during deregistration. The default is 60s. Setting this to a larger value gives targets more time to gracefully drain connections, but increases the time required for new deployments. Range 0s-3600s.
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-target-container" href="#http-additional-rules-target-container" class="field">`target_container`</a> <span class="type">String</span>  
    A sidecar container that requests are routed to instead of the main service container.  
    If the target container's port is set to `443`, then the protocol is set to `HTTPS` so that the load balancer establishes
    TLS connections with the Fargate tasks using certificates that you install on the target container.
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-target-port" href="#http-additional-rules-target-port" class="field">`target_port`</a> <span class="type">String</span>  
    The container port that receives traffic. Specify this field if the container port is different from `image.port` for the main container or `sidecar.port` for the sidecar containers.
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-stickiness" href="#http-additional-rules-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
    Indicates whether sticky sessions are enabled.
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-allowed-source-ips" href="#http-additional-rules-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
    CIDR IP addresses permitted to access your service.
    ```yaml
    http:
      additional_rules:
        - allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
    ```
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-alias" href="#http-additional-rules-alias" class="field">`alias`</a> <span class="type">String or Array of Strings or Array of Maps</span>  
    HTTPS domain alias of your service.
    ```yaml
    # String version.
    http:
      additional_rules:
        - alias: example.com
    # Alternatively, as an array of strings.
    http:
      additional_rules:
        - alias: ["example.com", "v1.example.com"]
    # Alternatively, as an array of maps.
    http:
      additional_rules:
        - alias:
            - name: example.com
              hosted_zone: Z0873220N255IR3MTNR4
            - name: v1.example.com
              hosted_zone: AN0THE9H05TED20NEID
    ```
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-hosted-zone" href="#http-additional-rules-hosted-zone" class="field">`hosted_zone`</a> <span class="type">String</span>  
    ID of your existing hosted zone; can only be used with `http.alias` and `http.additional_rules.alias`. If you have an environment with imported certificates, you can specify the hosted zone into which Copilot should insert the A record once the load balancer is created.
    ```yaml
    http:
      additional_rules:
        - alias: example.com
          hosted_zone: Z0873220N255IR3MTNR4
    # Also see http.alias array of maps example, above.
    ```
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-redirect-to-https" href="#http-additional-rules-redirect-to-https" class="field">`redirect_to_https`</a> <span class="type">Boolean</span>  
    Automatically redirect the Application Load Balancer from HTTP to HTTPS. By default it is `true`.
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-version" href="#http-additional-rules-version" class="field">`version`</a> <span class="type">String</span>  
    The HTTP(S) protocol version. Must be one of `'grpc'`, `'http1'`, or `'http2'`. If omitted, then `'http1'` is assumed.
    If using gRPC, please note that a domain must be associated with your application.