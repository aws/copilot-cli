# svc init
```console
$ copilot svc init
```

## コマンドの概要

`copilot svc init` は、コードを実行するために新しい [Service](../concepts/services.ja.md) を作成します。

コマンドを実行すると、 CLI はローカルの `copilot` ディレクトリに Service 名のサブディレクトリを作成し、そこに [Manifest ファイル](../manifest/overview.ja.md)を作成します。自由に Manifest ファイルを更新し、Service のデフォルト設定を変更できます。また CLI は全ての [Environment](../concepts/environments.ja.md) からプル可能にするポリシーをもつ ECR リポジトリをセットアップします。

そして Service は CLI からトラックするため AWS System Manager Parameter Store に登録されます。

その後、既にセットアップされた Environment がある場合は copilot deploy を実行して Service をデプロイできます。

## フラグ

```
Flags
      --allow-downgrade                Optional. Allow using an older version of Copilot to update Copilot components
                                       updated by a newer version of Copilot.
  -a, --app string                     Name of the application.
  -d, --dockerfile string              Path to the Dockerfile.
                                       Cannot be specified with --image.
  -h, --help                           help for init
  -i, --image string                   The location of an existing Docker image.
                                       Cannot be specified with --dockerfile or --build-context.
      --ingress-type string            Required for a Request-Driven Web Service. Allowed source of traffic to your service.
                                       Must be one of Environment or Internet.
  -n, --name string                    Name of the service.
      --no-subscribe                   Optional. Turn off selection for adding subscriptions for worker services.
      --port uint16                    The port on which your service listens.
      --sources stringArray            List of relative paths to source directories or files.
                                       Must be specified with '--svc-type "Static Site"'.
      --subscribe-topics stringArray   Optional. SNS topics to subscribe to from other services in your application.
                                       Must be of format '<svcName>:<topicName>'.
  -t, --svc-type string                Type of service to create. Must be one of:
                                       "Request-Driven Web Service", "Load Balanced Web Service", "Backend Service", "Worker Service", "Static Site".
```

"frontend" として Load Balanced Web Service を作成するには、次のように実行します。

`$ copilot svc init --name frontend --svc-type "Load Balanced Web Service" --dockerfile ./frontend/Dockerfile`

## 出力例

![Running copilot svc init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-init.svg?sanitize=true)
