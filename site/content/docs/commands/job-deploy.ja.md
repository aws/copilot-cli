# job deploy
```console
$ copilot job deploy
```

## 概要

`job deploy` はローカルのコードと設定を元に Job をデプロイします。

`job deploy` は以下のステップを実行します。

1. `image.build` が Manifest に存在する場合
    1. ローカルの Dockerfile をビルドしてコンテナイメージを作成
    2. `--tag` あるいは git ディレクトリにいる場合は最新の git ハッシュ値を使ってコンテナイメージにタグ付け
    3. ECR にコンテナイメージをプッシュ
2. Manifest ファイルと Addon をまとめて CloudFormation テンプレートにパッケージ
3. ECS のタスク定義と Job を作成/更新

## フラグ

```
      --allow-downgrade                Optional. Allow using an older version of Copilot to update Copilot components
                                       updated by a newer version of Copilot.
  -a, --app string                     Name of the application.
      --detach                         Optional. Skip displaying CloudFormation deployment progress.
      --diff                           Compares the generated CloudFormation template to the deployed stack.
  -e, --env string                     Name of the environment.
  -h, --help                           help for deploy
  -n, --name string                    Name of the job.
      --no-rollback                    Optional. Disable automatic stack
                                       rollback in case of deployment failure.
                                       We do not recommend using this flag for a
                                       production environment.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The tag for the container images Copilot builds from Dockerfiles.
```

!!!info
`--no-rollback` フラグは、サービスのダウンタイムを招く可能性があるため、本番環境にデプロイする場合は ***お勧めしません***。
スタックの自動ロールバックが無効な場合にデプロイに失敗すると、次のデプロイの前に AWS コンソールまたは AWS CLI を使用してスタックのロールバックを手動で開始する必要がある場合があります。

## 実行例

"report-gen" という Job を "test" Environment にデプロイします。
```console
$ copilot job deploy --name report-gen --env test
```

追加のリソースタグを付与して Job をデプロイします。
```console
$ copilot job deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`
```

`--diff` を使用して、デプロイを実行する前に、変更される内容を確認します。
```console
$ copilot job deploy --diff
~ Resources:
    ~ TaskDefinition:
        ~ Properties:
            ~ ContainerDefinitions:
                ~ - (changed item)
                  ~ Environment:
                      (4 unchanged items)
                      + - Name: LOG_LEVEL
                      +   Value: "info"

Continue with the deployment? (y/N)
```

!!!info "`copilot job package --diff`"
    デプロイを実行する必要がなく、差分だけを確認したい場合があります。
    `copilot job package --diff` は差分を表示してコマンドが終了します。
