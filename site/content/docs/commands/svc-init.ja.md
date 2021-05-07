# svc init
```bash
$ copilot svc init
```

## コマンドの概要

`copilot svc init` は、コードを実行するために新しい [Service](../concepts/services.md) を作成します。 

コマンドを実行すると、 CLI はローカルの `copilot` ディレクトリに Application 名のサブディレクトリを作成し、そこに [Manifest ファイル](../manifest/overview.md) を作成します。自由に Manifest ファイルを更新し、Service のデフォルト設定を変更できます。また CLI は全ての [Environment](../concepts/environments.md) からプル可能にするポリシーをもつ ECR リポジトリをセットアップします。

そして Service は CLI からトラックするため AWS System Manager Parameter Store に登録されます。

その後、既にセットアップされた Environment がある場合は `copilot deploy` を実行して Service をデプロイできます。

## フラグ

```bash
Required Flags
  -d, --dockerfile string   Path to the Dockerfile.
  -n, --name string         Name of the service.
  -t, --svc-type string     Type of service to create. Must be one of:
                            "Load Balanced Web Service", "Backend Service"

Load Balanced Web Service Flags
      --port uint16   Optional. The port on which your service listens.

Backend Service Flags
      --port uint16   Optional. The port on which your service listens.
```

各 Service type には共通の必須フラグの他に、独自のオプションフラグと必須フラグがあります。"frontend" として Load Balanced Web Service を作成するには、次のように実行します


`$ copilot svc init --name frontend --app-type "Load Balanced Web Service" --dockerfile ./frontend/Dockerfile`

## 出力例

![Running copilot svc init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-init.svg?sanitize=true)
