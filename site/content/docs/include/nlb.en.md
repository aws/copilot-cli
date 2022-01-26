<div class="separator"></div>

<a id="nlb" href="#nlb" class="field">`nlb`</a> <span class="type">Map</span>  
The nlb section contains parameters related to integrating your service with a Network Load Balancer.

The Network Load Balancer is only enabled if you specify the `nlb` field. Note that for a Load-Balanced Web Service,
at least one of Application Load Balancer and Network Load Balancer must be enabled.

<span class="parent-field">nlb.</span><a id="nlb-port" href="#nlb-port" class="field">`port`</a> <span class="type">String</span>  
Required. The port and protocol for the Network Load Balancer to listen on. 

Accepted protocols include `tcp` and `tls`. If the protocol is not specified, `tcp` is used by default. For example:
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

