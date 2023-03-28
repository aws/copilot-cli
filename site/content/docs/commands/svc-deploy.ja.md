# svc deploy
```console
$ copilot svc deploy
```

## コマンドの概要

`copilot svc deploy` は、ローカルのコードや設定を元にデプロイします。

Service デプロイの手順は以下の通りです。

1. ローカルの Dockerfile をビルドしてコンテナイメージを作成
2. `--tag` の値、または最新の git sha (git ディレクトリで作業している場合) を利用してタグ付け
3. コンテナイメージを ECR にプッシュ
4. Manifest ファイルと Addon を CloudFormation にパッケージ
5. ECS タスク定義とサービスを作成 / 更新

## フラグ

```
  -a, --app string                     Name of the application.
      --diff                           Compares the generated CloudFormation template to the deployed stack.
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
      --tag string                     Optional. The container image tag.
```

!!!info
    `--no-rollback` フラグは production 環境へのデプロイには **推奨されません**。 サービスダウンタイムが発生する可能性があります。
    自動的なスタックロールバックが無効化されている状況でデプロイに失敗した場合、次のデプロイを行う前に、AWS Console や AWS CLI を利用して、手動でスタックを開始する必要がある場合があります。