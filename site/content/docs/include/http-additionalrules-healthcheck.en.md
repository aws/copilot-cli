<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-healthcheck" href="#http-additional-rules-healthcheck" class="field">`healthcheck`</a> <span class="type">String or Map</span>  
    If you specify a string, Copilot interprets it as the path exposed in your container to handle target group health check requests. The default is "/".
    ```yaml
    http:
      additional_rules:
        - healthcheck: '/'
    ```
    You can also specify healthcheck as a map:
    ```yaml
    http:
      additional_rules:
        - healthcheck:
            path: '/'
            port: 8080
            success_codes: '200'
            healthy_threshold: 3
            unhealthy_threshold: 2
            interval: 15s
            timeout: 10s
    ```
    
<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-path" href="#http-additional-rules-healthcheck-path" class="field">`path`</a> <span class="type">String</span>  
    The destination that the health check requests are sent to.
    
<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-port" href="#http-additional-rules-healthcheck-port" class="field">`port`</a> <span class="type">Integer</span>  
    The port that the health check requests are sent to. The default is [`image.port`](./#image-port), or the port exposed by [`http.target_container`](./#http-target-container), if set.  
    If the port exposed is `443`, then the health check protocol is automatically set to HTTPS.
    
<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-success-codes" href="#http-additional-rules-healthcheck-success-codes" class="field">`success_codes`</a> <span class="type">String</span>  
    The HTTP status codes that healthy targets must use when responding to an HTTP health check. You can specify values between 200 and 499. You can specify multiple values (for example, "200,202") or a range of values (for example, "200-299"). The default is 200.
    
<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-healthy-threshold" href="#http-additional-rules-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
    The number of consecutive health check successes required before considering an unhealthy target healthy. The default is 5. Range: 2-10.
    
<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-unhealthy-threshold" href="#http-additional-rules-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
    The number of consecutive health check failures required before considering a target unhealthy. The default is 2. Range: 2-10.
    
<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-interval" href="#http-additional-rules-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
    The approximate amount of time, in seconds, between health checks of an individual target. The default is 30s. Range: 5s–300s.
    
<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-timeout" href="#http-additional-rules-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
    The amount of time, in seconds, during which no response from a target means a failed health check. The default is 5s. Range 5s-300s.