<div class="separator"></div>

<a id="deployment" href="#deployment" class="field">`deployment`</a> <span class="type">Map</span>  
The deployment section contains parameters relating to the ECS deployment.

<span class="parent-field">deployment.</span><a id="deployment-rolling" href="#deployment-rolling" class="field">`rolling`</a> <span class="type">String</span>  
Rolling deployment strategy. Valid values are

- default: Run new tasks before stopping all the old ones.
- recreate: Stop all running tasks and then spin up new tasks.
