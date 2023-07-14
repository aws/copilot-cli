<div class="separator"></div>

<a id="nlb" href="#nlb" class="field">`nlb`</a> <span class="type">Map</span>  
The nlb section contains parameters related to integrating your service with a Network Load Balancer.

The Network Load Balancer is enabled only if you specify the `nlb` field. Note that for a Load-Balanced Web Service,
at least one of Application Load Balancer and Network Load Balancer must be enabled.

<span class="parent-field">nlb.</span><a id="nlb-port" href="#nlb-port" class="field">`port`</a> <span class="type">String</span>  
Required. The port and protocol for the Network Load Balancer to listen on. 

Accepted protocols include `tcp`, `udp` and `tls`. If the protocol is not specified, `tcp` is used by default. For example:
```yaml
nlb:
  port: 80
```
will listen on port 80 for `tcp` requests. This is the same as 
```yaml
nlb:
  port: 80/tcp
```

You can easily enable TLS termination. For example:
```yaml
nlb:
  port: 443/tls
```

<span class="parent-field">nlb.</span><a id="nlb-healthcheck" href="#nlb-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
Specify the health check configuration for your Network Load Balancer.
```yaml
nlb:
  healthcheck:
    port: 80
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
```

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-port" href="#nlb-healthcheck-port" class="field">`port`</a> <span class="type">String</span>  
The port that the health check requests are sent to. Specify this if your health check should be performed on a different port than the container target port.

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-healthy-threshold" href="#nlb-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
The number of consecutive health check successes required before considering an unhealthy target healthy. The default is 3. Range: 2-10.

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-unhealthy-threshold" href="#nlb-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
The number of consecutive health check failures required before considering a target unhealthy. The default is 3. Range: 2-10.

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-grace-period" href="#nlb-healthcheck-grace-period" class="field">`grace_period`</a> <span class="type">Duration</span>  
The amount of time to ignore failing target group healthchecks on container start. The default is 60s. This can be useful to fix deployment issues for containers which take a while to become healthy and begin listening for incoming connections, or to speed up deployment of containers guaranteed to start quickly.

!!! info
    Per the [docs](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/target-group-health-checks.html) at the time of this writing, 'unhealthy threshold' is required to be equal to 'healthy threshold' for a Network Load Balancer.

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-interval" href="#nlb-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
The approximate amount of time, in seconds, between health checks of an individual target. The value can be 10s or 30s. The default is 30s. 

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-timeout" href="#nlb-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
The amount of time, in seconds, during which no response from a target means a failed health check. The default is 10s.

<span class="parent-field">nlb.</span><a id="nlb-target-container" href="#nlb-target-container" class="field">`target_container`</a> <span class="type">String</span>  
A sidecar container that takes the place of a service container.

<span class="parent-field">nlb.</span><a id="nlb-target-port" href="#nlb-target-port" class="field">`target_port`</a> <span class="type">Integer</span>  
The container port that receives traffic. Specify this field if the container port is different from `nlb.port`, the listener port.

<span class="parent-field">nlb.</span><a id="nlb-ssl-policy" href="#nlb-ssl-policy" class="field">`ssl_policy`</a> <span class="type">String</span>  
The security policy that defines which protocols and ciphers are supported. To learn more, see [this doc](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/create-tls-listener.html#describe-ssl-policies).

<span class="parent-field">nlb.</span><a id="nlb-stickiness" href="#nlb-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
Indicates whether sticky sessions are enabled.

<span class="parent-field">nlb.</span><a id="nlb-alias" href="#nlb-alias" class="field">`alias`</a> <span class="type">String or Array of Strings</span>  
Domain aliases for your service.
```yaml
# String version.
nlb:
  alias: example.com
# Alteratively, as an array of strings.
nlb:
  alias: ["example.com", "v1.example.com"]
```
<span class="parent-field">nlb.</span><a id="nlb-additional-listeners" href="#nlb-additional-listeners" class="field">`additional_listeners`</a> <span class="type">Array of Maps</span>  
Configure multiple NLB listeners.

{% include 'nlb-additionallisteners.en.md' %}