List of all available properties for a Copilot pipeline manifest. To learn more about pipelines, see the [Pipelines](../concepts/pipelines.en.md) concept page.

???+ note "Sample continuous delivery pipeline manifests"

    === "Release workloads"
        ```yaml
        # The "app-pipeline" will deploy all the services and jobs in the user/repo
        # to the "test" and "prod" environments.
        name: app-pipeline
    
        source:
          provider: GitHub
          properties:
            branch: main
            repository: https://github.com/user/repo
            # Optional: specify the name of an existing CodeStar Connections connection.
            # connection_name: a-connection
    
        build:
          image: aws/codebuild/amazonlinux2-x86_64-standard:4.0
          # additional_policy: # Add additional permissions while building your container images and templates.
    
        stages: 
          - # By default all workloads are deployed concurrently within a stage.
            name: test
            pre_deployments:
              db_migration:
                buildspec: ./buildspec.yml
            test_commands:
              - make integ-test
              - echo "woo! Tests passed"
          -
            name: prod
            requires_approval: true
        ```

    === "Control order of deployments"

        ```yaml
        # Alternatively, you can control the order of stack deployments in a stage. 
        # See https://aws.github.io/copilot-cli/blogs/release-v118/#controlling-order-of-deployments-in-a-pipeline
        name: app-pipeline
    
        source:
          provider: Bitbucket
          properties:
            branch: main
            repository:  https://bitbucket.org/user/repo
    
        stages:
          - name: test
            deployments:
              orders:
              warehouse:
              frontend:
                depends_on: [orders, warehouse]
          - name: prod
            require_approval: true
            deployments:
              orders:
              warehouse:
              frontend:
                depends_on: [orders, warehouse]
        ```

    === "Release environments"

        ```yaml
        # Environment manifests changes can also be released with a pipeline.
        name: env-pipeline
    
        source:
          provider: CodeCommit
          properties:
            branch: main
            repository: https://git-codecommit.us-east-2.amazonaws.com/v1/repos/MyDemoRepo
    
        stages:
          - name: test
            deployments:
              deploy-env:
                template_path: infrastructure/test.env.yml
                template_config: infrastructure/test.env.params.json
                stack_name: app-test
          - name: prod
            deployments:
              deploy-prod:
                template_path: infrastructure/prod.env.yml
                template_config: infrastructure/prod.env.params.json
                stack_name: app-prod
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
The name of the branch in your repository that triggers the pipeline. Copilot autofills this field with your current local branch.

<span class="parent-field">source.properties.</span><a id="source-properties-repository" href="#source-properties-repository" class="field">`repository`</a> <span class="type">String</span>  
The URL of your repository.

<span class="parent-field">source.properties.</span><a id="source-properties-connection-name" href="#source-properties-connection-name" class="field">`connection_name`</a> <span class="type">String</span>  
The name of an existing CodeStar Connections connection. If omitted, Copilot will generate a connection for you.

<span class="parent-field">source.properties.</span><a id="source-properties-output-artifact-format" href="#source-properties-output-artifact-format" class="field">`output_artifact_format`</a> <span class="type">String</span>  
Optional. The output artifact format. Values can be either `CODEBUILD_CLONE_REF` or `CODE_ZIP`. If omitted, the default is `CODE_ZIP`.

!!! info
    This property is not available for pipelines with [GitHub version 1](https://docs.aws.amazon.com/codepipeline/latest/userguide/appendix-github-oauth.html) source actions, which use `access_token_secret`. 

<div class="separator"></div>

<a id="build" href="#build" class="field">`build`</a> <span class="type">Map</span>  
Configuration for CodeBuild project.

<span class="parent-field">build.</span><a id="build-image" href="#build-image" class="field">`image`</a> <span class="type">String</span>  
The URI that identifies the Docker image to use for this build project. As of now, `aws/codebuild/amazonlinux2-x86_64-standard:4.0` is used by default.

<span class="parent-field">build.</span><a id="build-buildspec" href="#build-buildspec" class="field">`buildspec`</a> <span class="type">String</span>  
Optional. The path to a buildspec file, relative to the project root, to use for this build project. By default, Copilot will generate one for you, located at `copilot/pipelines/[your pipeline name]/buildspec.yml`.

<span class="parent-field">build.</span><a id="build-additional-policy" href="#build-additional-policy" class="field">`additional_policy.`</a><a id="policy-document" href="#policy-document" class="field">`PolicyDocument`</a> <span class="type">Map</span>  
Optional. Specify an additional policy document to add to the build project role.
The additional policy document can be specified in a map in YAML, for example:
```yaml
build:
  additional_policy:
    PolicyDocument:
      Version: 2012-10-17
      Statement:
        - Effect: Allow
          Action:
            - ecr:GetAuthorizationToken
          Resource: '*'
```
or alternatively as JSON:
```yaml
build:
  additional_policy:
    PolicyDocument: 
      {
        "Statement": [
          {
            "Action": ["ecr:GetAuthorizationToken"],
            "Effect": "Allow",
            "Resource": "*"
          }
        ],
        "Version": "2012-10-17"
      }
```

<div class="separator"></div>

<a id="stages" href="#stages" class="field">`stages`</a> <span class="type">Array of Maps</span>  
Ordered list of environments that your pipeline will deploy to.

<span class="parent-field">stages.</span><a id="stages-name" href="#stages-name" class="field">`name`</a> <span class="type">String</span>  
The name of an environment to deploy your services to.

<span class="parent-field">stages.</span><a id="stages-approval" href="#stages-approval" class="field">`requires_approval`</a> <span class="type">Boolean</span>  
Optional. Indicates whether to add a manual approval step before the deployment (or the pre-deployment actions, if you have added any). Defaults to `false`.

<span class="parent-field">stages.</span><a id="stages-predeployments" href="#stages-predeployments" class="field">`pre_deployments`</a> <span class="type">Map</span> <span class="version">Added in [v1.30.0](../../blogs/release-v130.en.md#deployment-actions)</span>  
Optional. Add actions to be executed before deployments.
```yaml
stages:
  - name: <env name>
    pre_deployments:
      <action name>:
        buildspec: <path to local buildspec>
        depends_on: [<other action's name>, ...]
```
<span class="parent-field">stages.pre_deployments.</span><a id="stages-predeployments-name" href="#stages-predeployments-name" class="field">`<name>`</a> <span class="type">Map</span> <span class="version">Added in [v1.30.0](../../blogs/release-v130.en.md#deployment-actions)</span>  
Name of the pre-deployment action.

<span class="parent-field">stages.pre_deployments.`<name>`.</span><a id="stages-predeployments-buildspec" href="#stages-predeployments-buildspec" class="field">`buildspec`</a> <span class="type">String</span> <span class="version">Added in [v1.30.0](../../blogs/release-v130.en.md#deployment-actions)</span>  
The path to a buildspec file, relative to the project root, to use for this build project.

<span class="parent-field">stages.pre_deployments.`<name>`.</span><a id="stages-predeployments-dependson" href="#stages-predeployments-dependson" class="field">`depends_on`</a> <span class="type">Array of Strings</span> <span class="version">Added in [v1.30.0](../../blogs/release-v130.en.md#deployment-actions)</span>  
Optional. Names of other pre-deployment actions that should be deployed prior to deploying this action. Defaults to no dependencies.

!!! info
    For more on pre- and post-deployments, see the [v1.30.0 blog post](../../blogs/release-v130.en.md) and the [Pipelines](../concepts/pipelines.en.md) page.

<span class="parent-field">stages.</span><a id="stages-deployments" href="#stages-deployments" class="field">`deployments`</a> <span class="type">Map</span>  
Optional. Control which CloudFormation stacks to deploy and their order.  
The `deployments` dependencies are specified in a map of the form:
```yaml
stages:
  - name: test
    deployments:
      <service or job name>:
      <other service or job name>:
        depends_on: [<name>, ...]
```

For example, if your git repository has the following layout:
```
copilot
├── api
│   └── manifest.yml
└── frontend
    └── manifest.yml
```

And you'd like to control the order of your deployments, such that `api` is deployed before `frontend`, then you can configure your stage as follows:
```yaml
stages:
  - name: test
    deployments:
      api:
      frontend:
        depends_on:
          - api
```
You can also limit which microservices to release part of your pipeline. In the following manifest, we're specifying to deploy only `api` and not `frontend`:
```yaml
stages:
  - name: test
    deployments:
      api:
```

Finally, if `deployments` isn't specified, by default Copilot will deploy all your services and job in the git repository in parallel.

<span class="parent-field">stages.deployments.</span><a id="stages-deployments-name" href="#stages-deployments-name" class="field">`<name>`</a> <span class="type">Map</span>  
Name of the job or service to deploy.  

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-dependson" href="#stages-deployments-dependson" class="field">`depends_on`</a> <span class="type">Array of Strings</span>  
Optional. Name of other jobs or services that should be deployed prior to deploying this microservice. Defaults to no dependencies.  

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-stackname" href="#stages-deployments-stackname" class="field">`stack_name`</a> <span class="type">String</span>  
Optional. Name of the stack to create or update. Defaults to `<app name>-<stage name>-<deployment name>`.  
For example, if your application is called `demo`, stage name is `test`, and your service name is `frontend` then the stack name will be `demo-test-frontend`.  

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-templatepath" href="#stages-deployments-templatepath" class="field">`template_path`</a> <span class="type">String</span>  
Optional. Path to the CloudFormation template generated during the `build` phase. Defaults to `infrastructure/<deployment name>-<stage name>.yml`.

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-templateconfig" href="#stages-deployments-templatepath" class="field">`template_config`</a> <span class="type">String</span>  
Optional. Path to the CloudFormation template configuration generated during the `build` phase. Defaults to `infrastructure/<deployment name>-<stage name>.params.json`.

<span class="parent-field">stages.</span><a id="stages-postdeployments" href="#stages-postdeployments" class="field">`post_deployments`</a> <span class="type">Map</span><span class="version">Added in [v1.30.0](../../blogs/release-v130.en.md#deployment-actions)</span>  
Optional. Add actions to be executed after deployments. Mutually exclusive with `stages.test_commands`.
```yaml
stages:
  - name: <env name>
    post_deployments:
      <action name>:
        buildspec: <path to local buildspec>
        depends_on: [<other action's name>, ...]
```
<span class="parent-field">stages.post_deployments.</span><a id="stages-postdeployments-name" href="#stages-postdeployments-name" class="field">`<name>`</a> <span class="type">Map</span> <span class="version">Added in [v1.30.0](../../blogs/release-v130.en.md#deployment-actions)</span>  
Name of the post-deployment action.

<span class="parent-field">stages.post_deployments.`<name>`.</span><a id="stages-postdeployments-buildspec" href="#stages-postdeployments-buildspec" class="field">`buildspec`</a> <span class="type">String</span> <span class="version">Added in [v1.30.0](../../blogs/release-v130.en.md#deployment-actions)</span>  
The path to a buildspec file, relative to the project root, to use for this build project.

<span class="parent-field">stages.post_deployments.`<name>`.</span><a id="stages-postdeployments-depends_on" href="#stages-postdeployments-dependson" class="field">`depends_on`</a> <span class="type">Array of Strings</span> <span class="version">Added in [v1.30.0](../../blogs/release-v130.en.md#deployment-actions)</span>   
Optional. Names of other post-deployment actions that should be deployed prior to deploying this action. Defaults to no dependencies.

<span class="parent-field">stages.</span><a id="stages-test-cmds" href="#stages-test-cmds" class="field">`test_commands`</a> <span class="type">Array of Strings</span>  
Optional. Commands to run integration or end-to-end tests after deployment. Defaults to no post-deployment validations. Mutually exclusive with `stages.post_deployment`.
