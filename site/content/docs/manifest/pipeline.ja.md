以下は Copilot Pipeline の Manifest で利用できるすべてのプロパティのリストです。[Pipeline の概念](../concepts/pipelines.ja.md)説明のページも合わせてご覧ください。

???+ note "継続的デリバリー Pipeline のサンプル Manifest"

    === "Release workloads"
        ```yaml
        # "app-pipeline" は、user/repo にあるすべての Service や Job を
        # "test" や "prod" Environment にデプロイします。
        name: app-pipeline
    
        source:
          provider: GitHub
          properties:
            branch: main
            repository: https://github.com/user/repo
            # オプション。既存の CodeStar Connection の接続名を指定します。
            # connection_name: a-connection
    
        build:
          image: aws/codebuild/amazonlinux2-x86_64-standard:4.0
          # additional_policy: # コンテナイメージやテンプレートを構築する際に、権限を追加することができます。
    
        stages: 
          - # デフォルトでは、すべてのワークロードはステージ内で同時にデプロイされます。
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
        # また、ステージ内のスタックデプロイの順番を制御することも可能です。
        # https://aws.github.io/copilot-cli/blogs/release-v118/#controlling-order-of-deployments-in-a-pipeline を参照してください。
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
        # Environment Manifest の変更も、Pipeline でリリースすることができます。
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
Pipeline 名。

<div class="separator"></div>

<a id="version" href="#version" class="field">`version`</a> <span class="type">String</span>  
テンプレートのスキーマバージョン。現在サポートされているバージョンは `1` だけです。

<div class="separator"></div>

<a id="source" href="#source" class="field">`source`</a> <span class="type">Map</span> 
Pipeline のトリガーに関する設定。

<span class="parent-field">source.</span><a id="source-provider" href="#source-provider" class="field">`provider`</a> <span class="type">String</span>  
プロバイダー名。現在 `GitHub` 、 `Bitbucket` そして `CodeCommit` がサポートされています。

<span class="parent-field">source.</span><a id="source-properties" href="#source-properties" class="field">`properties`</a> <span class="type">Map</span>  
Pipeline のトリガーに関するプロバイダー固有の設定。

<span class="parent-field">source.properties.</span><a id="source-properties-ats" href="#source-properties-ats" class="field">`access_token_secret`</a> <span class="type">String</span>  
Pipeline をトリガーするための GitHub アクセストークンを保持する AWS Secrets Manager シークレットの名前。
(プロバイダーが GitHub で、個人のアクセストークンを利用して Pipeline を作成した場合)
!!! info
    Copilot v1.4.0 から GitHub リポジトリをソースにする場合のアクセストークンは不要になりました。代わりに Copilot は [AWS CodeStar の GitHub への接続](https://docs.aws.amazon.com/ja_jp/codepipeline/latest/userguide/update-github-action-connections.html)を使って Pipeline をトリガーします。

<span class="parent-field">source.properties.</span><a id="source-properties-branch" href="#source-properties-branch" class="field">`branch`</a> <span class="type">String</span>  
Pipeline をトリガーするリポジトリのブランチ名。 Copilot は、このフィールドに現在のローカルブランチを自動的に入力します。

<span class="parent-field">source.properties.</span><a id="source-properties-repository" href="#source-properties-repository" class="field">`repository`</a> <span class="type">String</span>  
リポジトリの URL 。

<span class="parent-field">source.properties.</span><a id="source-properties-connection-name" href="#source-properties-connection-name" class="field">`connection_name`</a> <span class="type">String</span>  
既存の CodeStar Connections の接続名。指定しない場合 Copilot は接続を作成します。

<span class="parent-field">source.properties.</span><a id="source-properties-output-artifact-format" href="#source-properties-output-artifact-format" class="field">`output_artifact_format`</a> <span class="type">String</span>
任意項目。アーティファクトの出力形式です。`CODEBUILD_CLONE_REF` または `CODE_ZIP` を指定します。省略した場合、デフォルトの `CODE_ZIP` が利用されます。

!!! info
    このプロパティは、`access_token_secret` を使用する[GitHub version 1](https://docs.aws.amazon.com/ja_jp/codepipeline/latest/userguide/appendix-github-oauth.html) ソースアクションでは利用できません。

<div class="separator"></div>

<a id="build" href="#build" class="field">`build`</a> <span class="type">Map</span>  
CodeBuild プロジェクトに関する設定。

<span class="parent-field">build.</span><a id="build-image" href="#build-image" class="field">`image`</a> <span class="type">String</span>  
CodeBuild のビルドプロジェクトで利用する Docker イメージの URI。`aws/codebuild/amazonlinux2-x86_64-standard:4.0` がデフォルトで利用されます。

<span class="parent-field">build.</span><a id="build-buildspec" href="#build-buildspec" class="field">`buildspec`</a> <span class="type">String</span>
任意項目。このビルドプロジェクトで使用する、buildspec ファイルへのプロジェクトルートからの相対パスを指定します。デフォルトでは、作成したファイルは、 `copilot/pipelines/[your pipeline name]/buildspec.yml` に配置されています。

<span class="parent-field">build.</span><a id="build-additional-policy" href="#build-additional-policy" class="field">`additional_policy.`</a><a id="policy-document" href="#policy-document" class="field">`PolicyDocument`</a> <span class="type">Map</span>  
任意項目。ビルドプロジェクトロールに追加するポリシードキュメントを指定します。追加のポリシードキュメントは、以下の例のように YAML のマップに指定することができます。
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
Pipeline のデプロイ先である 1 つ以上の Environment をデプロイしたい順番に並べたリスト。

<span class="parent-field">stages.</span><a id="stages-name" href="#stages-name" class="field">`name`</a> <span class="type">String</span>  
Service をデプロイする Environment 名。

<span class="parent-field">stages.</span><a id="stages-approval" href="#stages-approval" class="field">`requires_approval`</a> <span class="type">Boolean</span>   
任意項目。デプロイの前に手動承認ステップを追加するかどうか (追加している場合はデプロイ前のアクションを追加するかどうか) を示します。デフォルトは `false` です。

<span class="parent-field">stages.</span><a id="stages-predeployments" href="#stages-predeployments" class="field">`pre_deployments`</a> <span class="type">Map</span> <span class="version">[v1.30.0](../../blogs/release-v130.ja.md#deployment-actions) にて追加</span>  
任意項目。デプロイ前に実行するアクションを追加します。 
```yaml
stages:
  - name: <env name>
    pre_deployments:
      <action name>:
        buildspec: <path to local buildspec>
        depends_on: [<other action's name>, ...]
```
<span class="parent-field">stages.pre_deployments.</span><a id="stages-predeployments-name" href="#stages-predeployments-name" class="field">`<name>`</a> <span class="type">Map</span> <span class="version">[v1.30.0](../../blogs/release-v130.ja.md#deployment-actions) にて追加</span>  
デプロイ前のアクションの名前。

<span class="parent-field">stages.pre_deployments.`<name>`.</span><a id="stages-predeployments-buildspec" href="#stages-predeployments-buildspec" class="field">`buildspec`</a> <span class="type">String</span> <span class="version">[v1.30.0](../../blogs/release-v130.ja.md#deployment-actions) にて追加</span>  
このビルドプロジェクトで使用する buildspec ファイルへのパスを、プロジェクトルートからの相対パスで指定します。

<span class="parent-field">stages.pre_deployments.`<name>`.</span><a id="stages-predeployments-dependson" href="#stages-predeployments-dependson" class="field">`depends_on`</a> <span class="type">Array of Strings</span> <span class="version">[v1.30.0](../../blogs/release-v130.ja.md#deployment-actions) にて追加</span>  
任意項目。このアクションをデプロイする前にデプロイする必要がある、他のデプロイ前アクションの名前。デフォルトでは依存関係はありません。

!!! info
    デプロイ前およびデプロイ後の詳細については、[v1.30.0 のブログ記事](../../blogs/release-v130.ja.md) および [Pipeline](../concepts/pipelines.ja.md) ページを参照してください。

<span class="parent-field">stages.</span><a id="stages-deployments" href="#stages-deployments" class="field">`deployments`</a> <span class="type">Map</span>  
任意項目。デプロイする CloudFormation スタックとその順序を制御します。
デプロイの依存関係は、次の形式の Map で指定されます。
```yaml
stages:
  - name: test
    deployments:
      <service or job name>:
      <other service or job name>:
        depends_on: [<name>, ...]
```

例えば、Git リポジトリのレイアウトが次のようになっているとします。
```
copilot
├── api
│   └── manifest.yml
└── frontend
    └── manifest.yml
```

また、`frontend` の前に `api` がデプロイされるようにデプロイの順序を制御したい場合は、ステージを次のように設定できます。
```yaml
stages:
  - name: test
    deployments:
      api:
      frontend:
        depends_on:
          - api
```
また、パイプラインの一部をリリースするマイクロサービスを制限することもできます。以下の Manifest では、`api` のみをデプロイし、`frontend` をデプロイしないよう指定しています。
```yaml
stages:
  - name: test
    deployments:
      api:
```

最後に、もし `deployments` が指定されていない場合、デフォルトでは Copilot は git リポジトリにあるすべての Service と Job を並行してデプロイします。

<span class="parent-field">stages.deployments.</span><a id="stages-deployments-name" href="#stages-deployments-name" class="field">`<name>`</a> <span class="type">Map</span>  
デプロイする Job または Service の名前。

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-dependson" href="#stages-deployments-dependson" class="field">`depends_on`</a> <span class="type">Array of Strings</span>   
任意項目。このマイクロサービスをデプロイする前にデプロイする必要がある他の Job または Service の名前。デフォルトでは依存関係はありません。

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-stackname" href="#stages-deployments-stackname" class="field">`stack_name`</a> <span class="type">String</span>  
任意項目。作成または更新するスタックの名前。デフォルトは `<app name>-<stage name>-<deployment name>` です。
たとえば、Application 名が `demo`、ステージ名が `test`、Service 名が `frontend` の場合、スタック名は `demo-test-frontend` になります。

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-templatepath" href="#stages-deployments-templatepath" class="field">`template_path`</a> <span class="type">String</span>  
任意項目。`build` フェーズで生成された CloudFormation テンプレートへのパス。デフォルトは `infrastructure/<deployment name>-<stage name>.yml` です。

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-templateconfig" href="#stages-deployments-templatepath" class="field">`template_config`</a> <span class="type">String</span>  
任意項目。`build` フェーズで生成された CloudFormation テンプレート設定へのパス。デフォルトは `infrastructure/<deployment name>-<stage name>.params.json` です。

<span class="parent-field">stages.</span><a id="stages-postdeployments" href="#stages-postdeployments" class="field">`post_deployments`</a> <span class="type">Map</span><span class="version">[v1.30.0](../../blogs/release-v130.ja.md#deployment-actions) にて追加</span>  
任意項目。デプロイ後に実行するアクションを追加します。`stages.test_commands` とは相互に排他的です。
```yaml
stages:
  - name: <env name>
    post_deployments:
      <action name>:
        buildspec: <path to local buildspec>
        depends_on: [<other action's name>, ...]
```
<span class="parent-field">stages.post_deployments.</span><a id="stages-postdeployments-name" href="#stages-postdeployments-name" class="field">`<name>`</a> <span class="type">Map</span> <span class="version">[v1.30.0](../../blogs/release-v130.ja.md#deployment-actions) にて追加</span>  
デプロイ後アクションの名前。

<span class="parent-field">stages.post_deployments.`<name>`.</span><a id="stages-postdeployments-buildspec" href="#stages-postdeployments-buildspec" class="field">`buildspec`</a> <span class="type">String</span> <span class="version">[v1.30.0](../../blogs/release-v130.ja.md#deployment-actions) にて追加</span>  
このビルドプロジェクトで使用する buildspec ファイルへのパスを、プロジェクトルートからの相対パスで指定します。

<span class="parent-field">stages.post_deployments.`<name>`.</span><a id="stages-postdeployments-depends_on" href="#stages-postdeployments-dependson" class="field">`depends_on`</a> <span class="type">Array of Strings</span> <span class="version">[v1.30.0](../../blogs/release-v130.ja.md#deployment-actions) にて追加</span>   
任意項目。このアクションをデプロイする前にデプロイする必要がある他のデプロイ後アクションの名前。 デフォルトでは依存関係はありません。

<span class="parent-field">stages.</span><a id="stages-test-cmds" href="#stages-test-cmds" class="field">`test_commands`</a> <span class="type">Array of Strings</span>   
任意項目。デプロイ後にインテグレーションテストまたは E2E テストを実行するコマンドです。デフォルトでは、デプロイ後の検証は行いません。'stages.post_deployment' とは相互に排他的です。
