# copilot deploy
```console
$ copilot deploy
```

## コマンドの概要 

このコマンドは、[`copilot svc deploy`](../commands/svc-deploy.ja.md) または [`copilot job deploy`](../commands/job-deploy.ja.md) の内部で利用されています。
`copilot deploy` の中で実行される各ステップは、`copilot svc deploy` や `copilot job deploy` で実行されるステップと同様です。

1. `image.build` が Manifest に存在する場合
    1. ローカルの Dockerfile からコンテナイメージを作成
    2. `--tag` で指定された値、または最新の git sha を利用してタグ付け(git 管理されている場合)
    3. コンテナイメージを ECR に対してプッシュ
2. Manifest ファイルと Addon を CloudFormation テンプレートにパッケージ
3. ECS タスク定義を作成/更新し、Job や Service を作成/更新

## フラグ

```
      --allow-downgrade                Optional. Allow using an older version of Copilot to update Copilot components
                                       updated by a newer version of Copilot.
  -a, --app string                     Name of the application.
  -e, --env string                     Name of the environment.
      --force                          Optional. Force a new service deployment using the existing image.
  -h, --help                           help for deploy
  -n, --name string                    Name of the service or job.
      --no-rollback bool               Optional. Disable automatic stack
                                       rollback in case of deployment failure.
                                       We do not recommend using this flag for a
                                       production environment.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The tag for the container images Copilot builds from Dockerfiles.
```

!!!info
`--no-rollback` フラグは、サービスのダウンタイムを招く可能性があるため、本番環境にデプロイする場合は ***お勧めしません*** 。
自動スタックロールバックが無効になっている場合に、デプロイに失敗すると、手動でスタックを開始する必要があります。次のデプロイの前に AWS コンソールまたは AWS CLI を利用してスタックのスタックロールバックを手動で開始する必要があります。

## 実行例

"frontend"という名前の Service を "test" Environment にデプロイします。
```console
 $ copilot deploy --name frontend --env test
```

"mailer"という名前の Job を、追加のリソースタグを付加して、"prod" Environment にデプロイします。
```console
$ copilot deploy -n mailer -e prod --resource-tags source/revision=bb133e7,deployment/initiator=manual
```
