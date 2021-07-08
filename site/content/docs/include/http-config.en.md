<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
The http section contains parameters related to integrating your service with an Application Load Balancer.

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>  
Requests to this path will be forwarded to your service. Each Load Balanced Web Service should listen on a unique path.

<span class="parent-field">http.</span><a id="http-healthcheck" href="#http-healthcheck" class="field">`healthcheck`</a> <span class="type">String or Map</span>  
If you specify a string, Copilot interprets it as the path exposed in your container to handle target group health check requests. The default is "/".
```yaml
http:
  healthcheck: '/'
```
You can also specify healthcheck as a map:
```yaml
http:
  healthcheck:
    path: '/'
    success_codes: '200'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
    deregistration_delay: 60s
```

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-path" href="#http-healthcheck-path" class="field">`path`</a> <span class="type">String</span>  
The destination that the health check requests are sent to.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-success-codes" href="#http-healthcheck-success-codes" class="field">`success_codes`</a> <span class="type">String</span>  
The HTTP status codes that healthy targets must use when responding to an HTTP health check. You can specify values between 200 and 499. You can specify multiple values (for example, "200,202") or a range of values (for example, "200-299"). The default is 200.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-healthy-threshold" href="#http-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
The number of consecutive health check successes required before considering an unhealthy target healthy. The default is 5. Range: 2-10.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
The number of consecutive health check failures required before considering a target unhealthy. The default is 2. Range: 2-10.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
The approximate amount of time, in seconds, between health checks of an individual target. The default is 30s. Range: 5s–300s.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
The amount of time, in seconds, during which no response from a target means a failed health check. The default is 5s. Range 5s-300s.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-deregistration-delay" href="#http-healthcheck-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>  
The amount of time to wait for targets to drain connections during deregistration. The default is 60s. Setting this to a larger value gives targets more time to gracefully drain connections, but increases the time required for new deployments.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-grace-period" href="#http-healthcheck-grace-period" class="field">`grace_period`</a> <span class="type">Duration</span>  
The amount of time to ignore failing target group healthchecks on container start. The default is 60s. This can be useful to fix deployment issues for containers which take a while to become healthy and begin listening for incoming connections, or to speed up deployment of containers guaranteed to start quickly.

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>  
A sidecar container that takes the place of a service container.

<span class="parent-field">http.</span><a id="http-stickiness" href="#http-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
Indicates whether sticky sessions are enabled.

<span class="parent-field">http.</span><a id="http-allowed-source-ips" href="#http-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
CIDR IP addresses permitted to access your service.
```yaml
http:
  allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
```

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String</span>  
HTTPS domain alias of your service.
