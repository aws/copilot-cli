<span class="parent-field">image.</span><a id="image-healthcheck" href="#image-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
Optional configuration for container health checks.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-cmd" href="#image-healthcheck-cmd" class="field">`command`</a> <span class="type">Array of Strings</span>  
The command to run to determine if the container is healthy.
The string array can start with `CMD` to execute the command arguments directly, or `CMD-SHELL` to run the command with the container's default shell.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-interval" href="#image-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
Time period between health checks, in seconds. Default is 10s.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-retries" href="#image-healthcheck-retries" class="field">`retries`</a> <span class="type">Integer</span>  
Number of times to retry before container is deemed unhealthy. Default is 2.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-timeout" href="#image-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
How long to wait before considering the health check failed, in seconds. Default is 5s.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-start-period" href="#image-healthcheck-start-period" class="field">`start_period`</a> <span class="type">Duration</span>  
Length of grace period for containers to bootstrap before failed health checks count towards the maximum number of retries. Default is 0s.
