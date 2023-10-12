# svc deploy
```console
$ copilot svc deploy
```

## コマンドの概要

`copilot svc deploy` は、ローカルのコードや設定を元にデプロイします。

Service デプロイの手順は以下の通りです。

1. `image.build` が Manifest に存在する場合
    1. ローカルの Dockerfile をビルドしてコンテナイメージを作成
    2. `--tag` の値、または最新の git sha (git ディレクトリで作業している場合) を利用してタグ付け
    3. コンテナイメージを ECR にプッシュ
2. Manifest ファイルと Addon をまとめて CloudFormation テンプレートにパッケージ
3. ECS タスク定義とサービスを作成 / 更新

## フラグ

```
      --allow-downgrade                Optional. Allow using an older version of Copilot to update Copilot components
                                       updated by a newer version of Copilot.
  -a, --app string                     Name of the application.
      --detach                         Optional. Skip displaying CloudFormation deployment progress.
      --diff                           Compares the generated CloudFormation template to the deployed stack.
      --diff-yes                       Skip interactive approval of diff before deploying.
  -e, --env string                     Name of the environment.
      --force                          Optional. Force a new service deployment using the existing image.
  -h, --help                           help for deploy
  -n, --name string                    Name of the service.
      --no-rollback                    Optional. Disable automatic stack
                                       rollback in case of deployment failure.
                                       We do not recommend using this flag for a
                                       production environment.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The tag for the container images Copilot builds from Dockerfiles.
```

!!!info
    `--no-rollback` フラグは production 環境へのデプロイには **推奨されません**。 サービスダウンタイムが発生する可能性があります。
    自動的なスタックロールバックが無効化されている状況でデプロイに失敗した場合、次のデプロイを行う前に、AWS Console や AWS CLI を利用して、手動でスタックを開始する必要がある場合があります。


## 実行例
`--diff` を使用して、デプロイを実行する前に、変更される内容を確認します。

```console
$ copilot svc deploy --diff
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

!!!info "`copilot svc package --diff`"
    デプロイを実行する必要がなく、差分だけを確認したい場合があります。
    `copilot svc package --diff` は差分を表示してコマンドが終了します。
