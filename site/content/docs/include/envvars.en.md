<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Key-value pairs that represent environment variables that will be passed to your service. Copilot will include a number of environment variables by default for you.

<span class="parent-field">variables.</span><a id="variables-from-cfn" href="#variables-from-cfn" class="field">`from_cfn`</a> <span class="type">String</span>  
The name of a [CloudFormation stack export](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html). 

<div class="separator"></div>

<a id="env_file" href="#env_file" class="field">`env_file`</a> <span class="type">String</span>  
The path to a file from the root of your workspace containing the environment variables to pass to the main container. For more information about the environment variable file, see [Considerations for specifying environment variable files](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations).
