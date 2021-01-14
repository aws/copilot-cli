List of all available properties for a Copilot pipeline manifest.
```yaml
# This YAML file defines the relationship and deployment ordering of your environments.

# The name of the pipeline
name: pipeline-sample-app-frontend

# The version of the schema used in this template
version: 1

# This section defines the source artifacts.
source:
  # The name of the provider that is used to store the source artifacts.
  provider: GitHub
  # Additional properties that further specifies the exact location
  # the artifacts should be sourced from. For example, the GitHub provider
  # has the following properties: repository, branch.
  properties:
    access_token_secret: github-token-sample-app
    branch: main
    repository: https://github.com/<user>/sample-app-frontend

# The deployment section defines the order the pipeline will deploy
# to your environments.
stages:
    - # The name of the environment to deploy to.
      name: test
      # Use test commands to validate your stage's deployment.
      test_commands:
        - make test
        - echo "woo! Tests passed"
    - # The name of the environment to deploy to.
      name: prod
      # Require a manual approval step before deployment.
      requires_approval: true
```
<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your pipeline.   

<div class="separator"></div>

<a id="version" href="#version" class="field">`version`</a> <span class="type">String</span>  
The schema version for the template. There is only one version, `1`, supported at the moment.

<div class="separator"></div>

<a id="source" href="#source" class="field">`source`</a> <span class="type">Map</span> 
Configuration for how your pipeline is triggered.

<span class="parent-field">source.</span><a id="source-provider" href="#source-provider" class="field">`provider`</a> <span class="type">String</span>  
The name of your provider. Currently, only `GitHub` and `CodeCommit` are supported.

<span class="parent-field">source.</span><a id="source-properties" href="#source-properties" class="field">`properties`</a> <span class="type">Map</span>  
Provider-specific configuration on how the pipeline is triggered.

<span class="parent-field">source.properties.</span><a id="source-properties-ats" href="#source-properties-ats" class="field">`access_token_secret`</a> <span class="type">String</span>  
The name of AWS Secrets Manager secret that holds the GitHub access token to trigger the pipeline if your provider is GitHub.

<span class="parent-field">source.properties.</span><a id="source-properties-branch" href="#source-properties-branch" class="field">`branch`</a> <span class="type">String</span>  
The name of the branch in your repository that triggers the pipeline. The default for GitHub is `main`; the default for CodeCommit is `master`.

<span class="parent-field">source.properties.</span><a id="source-properties-repository" href="#source-properties-repository" class="field">`repository`</a> <span class="type">String</span>  
The URL of your repository.

<div class="separator"></div>

<a id="stages" href="#stages" class="field">`stages`</a> <span class="type">Array of Maps</span>  
Ordered list of environments that your pipeline will deploy to.

<span class="parent-field">stages.</span><a id="stages-name" href="#stages-name" class="field">`name`</a> <span class="type">String</span>  
The name of an environment to deploy your services to.

<span class="parent-field">stages.</span><a id="stages-approval" href="#stages-approval" class="field">`requires_approval`</a> <span class="type">Boolean</span>   
Indicates whether to add a manual approval step before the deployment.

<span class="parent-field">stages.</span><a id="stages-test-cmds" href="#stages-test-cmds" class="field">`test_commands`</a> <span class="type">Array of Strings</span>   
Commands to run integration or end-to-end tests after deployment.  
