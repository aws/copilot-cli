List of all available properties for a Copilot pipeline manifest.

???+ note "Sample manifest for a pipeline triggered from a GitHub repo"

    ```yaml
    name: pipeline-sample-app-frontend
    version: 1

    source:
      provider: GitHub
      properties:
        branch: main
        repository: https://github.com/<user>/sample-app-frontend
        # Optional: specify the name of an existing CodeStar Connections connection.
        connection_name: MyPreexistingConnection
    
    stages:
        - 
          name: test
          test_commands:
            - make test
            - echo "woo! Tests passed"
        - 
          name: prod
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
The name of your provider. Currently, `GitHub`, `Bitbucket`, and `CodeCommit` are supported.

<span class="parent-field">source.</span><a id="source-properties" href="#source-properties" class="field">`properties`</a> <span class="type">Map</span>  
Provider-specific configuration on how the pipeline is triggered.

<span class="parent-field">source.properties.</span><a id="source-properties-ats" href="#source-properties-ats" class="field">`access_token_secret`</a> <span class="type">String</span>  
The name of AWS Secrets Manager secret that holds the GitHub access token to trigger the pipeline if your provider is GitHub and you created your pipeline with a personal access token.
!!! info
    As of AWS Copilot v1.4.0, the access token is no longer needed for GitHub repository sources. Instead, Copilot will trigger the pipeline [using AWS CodeStar connections](https://docs.aws.amazon.com/codepipeline/latest/userguide/update-github-action-connections.html). 

<span class="parent-field">source.properties.</span><a id="source-properties-branch" href="#source-properties-branch" class="field">`branch`</a> <span class="type">String</span>  
The name of the branch in your repository that triggers the pipeline. The default for GitHub is `main`; the default for Bitbucket and CodeCommit is `master`.

<span class="parent-field">source.properties.</span><a id="source-properties-repository" href="#source-properties-repository" class="field">`repository`</a> <span class="type">String</span>  
The URL of your repository.

<span class="parent-field">source.properties.</span><a id="source-properties-connection-name" href="#source-properties-connection-name" class="field">`connection_name`</a> <span class="type">String</span>  
The name of an existing CodeStar Connections connection. By default, Copilot will generate a connection for you.

<div class="separator"></div>

<a id="stages" href="#stages" class="field">`stages`</a> <span class="type">Array of Maps</span>  
Ordered list of environments that your pipeline will deploy to.

<span class="parent-field">stages.</span><a id="stages-name" href="#stages-name" class="field">`name`</a> <span class="type">String</span>  
The name of an environment to deploy your services to.

<span class="parent-field">stages.</span><a id="stages-approval" href="#stages-approval" class="field">`requires_approval`</a> <span class="type">Boolean</span>   
Indicates whether to add a manual approval step before the deployment.

<span class="parent-field">stages.</span><a id="stages-test-cmds" href="#stages-test-cmds" class="field">`test_commands`</a> <span class="type">Array of Strings</span>   
Commands to run integration or end-to-end tests after deployment.  
