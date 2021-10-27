以下は Copilot Pipeline の Manifest で利用できるすべてのプロパティのリストです。[Pipeline の概念](../concepts/pipelines.ja.md)説明のページも合わせてご覧ください。

???+ note "GitHub のリポジトリからトリガーされる Pipeline のサンプル Manifest"

```yaml
name: pipeline-sample-app-frontend
version: 1

source:
  provider: GitHub
  properties:
    branch: main
    repository: https://github.com/<user>/sample-app-frontend
    # オプション。既存の CodeStar Connection の接続名を指定します。
    connection_name: a-connection

build:
  image: aws/codebuild/amazonlinux2-x86_64-standard:3.0

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
Pipeline をトリガーするリポジトリのブランチ名。 GitHub と CodeCommit の場合デフォルトは `main` で Bitbucket の場合デフォルトは `master` です。

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
CodeBuild のビルドプロジェクトで利用する Docker イメージの URI。`aws/codebuild/amazonlinux2-x86_64-standard:3.0` がデフォルトで利用されます。

<div class="separator"></div>

<a id="stages" href="#stages" class="field">`stages`</a> <span class="type">Array of Maps</span>  
Pipeline のデプロイ先である 1 つ以上の Environment をデプロイしたい順番に並べたリスト。

<span class="parent-field">stages.</span><a id="stages-name" href="#stages-name" class="field">`name`</a> <span class="type">String</span>  
Service をデプロイする Environment 名。

<span class="parent-field">stages.</span><a id="stages-approval" href="#stages-approval" class="field">`requires_approval`</a> <span class="type">Boolean</span>   
デプロイの前に手動承認ステップを追加するかどうかの指定。

<span class="parent-field">stages.</span><a id="stages-test-cmds" href="#stages-test-cmds" class="field">`test_commands`</a> <span class="type">Array of Strings</span>   
デプロイ後にインテグレーションテストまたは E2E テストを実行するコマンド。
