# copilot deploy
```
$ copilot deploy
```

## コマンドの概要 

このコマンドは、[`copilot svc deploy`](../commands/svc-deploy.md) または [`copilot job deploy`](../commands/job-deploy.md) の内部で利用されています。
`copilot deploy` の中で実行される各ステップは、`copilot svc deploy` や `copilot job deploy` で実行されるステップと同様です。

1. ローカルの Dockerfile からコンテナイメージを作成
2. `--tag` で指定された値、または最新の git sha を利用してタグ付け(git 管理されている場合)
3. コンテナイメージを ECR に対してプッシュ
4. Manifest ファイルと Addon を CloudFormation テンプレートにパッケージ
5. ECS タスク定義を作成/更新し、Job や Service を作成/更新

## フラグ

```bash
  -a, --app string                     Name of the application.
  -e, --env string                     Name of the environment.
  -h, --help                           help for deploy
  -n, --name string                    Name of the service or job.
      --resource-tags stringToString   Optional. Labels with a key and value separated with commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The container image tag.
```

## 実行例

"frontend"という名前の Service を "test" Environment にデプロイします。
```bash
 $ copilot deploy --name frontend --env test
```

"mailer"という名前の Job を、追加のリソースタグを付加して、"prod" Environment にデプロイします。
```bash
$ copilot deploy -n mailer -e prod --resource-tags source/revision=bb133e7,deployment/initiator=manual
```