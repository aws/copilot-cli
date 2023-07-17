??? note "nlb.additional_listeners Map"
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-port" href="#nlb-additional-listeners-port" class="field">`port`</a> <span class="type">String</span>  
    Required. The additional port and protocol for the Network Load Balancer to listen on.
    
    Accepted protocols include `tcp`, `udp` and `tls`. If the protocol is not specified, `tcp` is used by default.
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-healthcheck" href="#nlb-additional-listeners-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
    Specify the health check configuration for your additional listener on the Network Load Balancer.
    ```yaml
    nlb:
      additional_listeners:
        - healthcheck:
            port: 80
            healthy_threshold: 3
            unhealthy_threshold: 2
            interval: 15s
            timeout: 10s
    ```
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-port" href="#nlb-additional-listeners-healthcheck-port" class="field">`port`</a> <span class="type">String</span>  
    The port that the health check requests are sent to. Specify this if your health check should be performed on a different port than the container target port.
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-healthy-threshold" href="#nlb-additional-listeners-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
    The number of consecutive health check successes required before considering an unhealthy target healthy. The default is 3. Range: 2-10.
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-unhealthy-threshold" href="#nlb-additional-listeners-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
    The number of consecutive health check failures required before considering a target unhealthy. The default is 3. Range: 2-10.
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-interval" href="#nlb-additional-listeners-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
    The approximate amount of time, in seconds, between health checks of an individual target. The value can be 10s or 30s. The default is 30s.
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-timeout" href="#nlb-additional-listeners-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
    The amount of time, in seconds, during which no response from a target means a failed health check. The default is 10s.
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-target-container" href="#nlb-additional-listeners-target-container" class="field">`target_container`</a> <span class="type">String</span>  
    A sidecar container that takes the place of a service container.
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-target-port" href="#nlb-additional-listeners-target-port" class="field">`target_port`</a> <span class="type">Integer</span>  
    The container port that receives traffic. Specify this field if the container port is different from `nlb.port`, the listener port.
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-ssl-policy" href="#nlb-additional-listeners-ssl-policy" class="field">`ssl_policy`</a> <span class="type">String</span>  
    The security policy that defines which protocols and ciphers are supported. To learn more, see [this doc](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/create-tls-listener.html#describe-ssl-policies).
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-stickiness" href="#nlb-additional-listeners-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
    Indicates whether sticky sessions are enabled.