# シークレット

シークレットは、OAuth トークン、シークレットキー、API キーなどの機密情報です。これらの情報はアプリケーションコードでは必要ですが、
ソースコードにコミットするべきではありません。AWS Copilot CLI では、シークレットは環境変数として渡されますが、その機密性のため扱いが異なります
 (詳細は[環境変数を使った開発](../developing/environment-variables.ja.md)を参照して下さい) 。

## シークレットの追加方法

シークレットを追加するには、シークレットを [AWS Systems Manager パラメータストア](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/systems-manager-parameter-store.html) (SSM) 、
または [AWS Secrets Manager](https://docs.aws.amazon.com/ja_jp/secretsmanager/latest/userguide/intro.html) に保存する必要があります。そして、SSM パラメータへの参照を [Manifest](../manifest/overview.ja.md) に追加します。

[`copilot secret init`](../commands/secret-init.ja.md) コマンドを利用することで、SSM に簡単に `SecureString` としてシークレットを作成できます！

!!! attention
    Request-Driven Web Service はシークレットの利用をサポートしていません。

## Copilot の外部で作成したシークレットの取り込み

## SSM の場合
Copilot の外部で作成したシークレットを持ち込みたい場合、そのシークレットに次の２つのタグを設定することを忘れないようにしてください。

| Key                     | Value                                                       |
| ----------------------- | ----------------------------------------------------------- |
| `copilot-application`   | このシークレットを利用したい Copilot Application 名              |
| `copilot-environment`   | このシークレットを利用したい Copilot Environment 名              |

上記の `copilot-application` と `copilot-environment` タグは、Copilot が持ち込みシークレットへのアクセスを適切に制御するために必要となります。

`GH_WEBHOOK_SECRET` という名前で値に `secretvalue1234` を持つ（適切にタグが設定された）SSM パラメータがあると仮定しましょう。このシークレットを Manifest ファイルから参照するには、次のような内容を Manifest に記述することになります。

```yaml
secrets:                      
  GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET  
```

更新された Manifest をデプロイすると、Service や Job は環境変数 `GITHUB_WEBHOOK_SECRET` にアクセスできるようになります。この環境変数には、SSM パラメータ `GH_WEBHOOK_SECRET` の値である `secretvalue1234` が格納されます。
これが機能するのは、ECS エージェントがタスクの開始時に SSM パラメータを解決し、環境変数を設定してくれるためです。

### Secrets Manager の場合
SSM と同様に、最初に Secrets Manager のシークレットに、`copilot-application` と `copilot-environment` のタグがあることを確認します。 

次の構成の Secrets Manager のシークレットがあるとします。

| Field  | Value                                                                                                                                                                 |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Name   | `demo/test/mysql`                                                                                                                                                     |
| ARN    | `arn:aws:secretsmanager:us-west-2:111122223333:secret:demo/test/mysql-Yi6mvL`                                                                                        |
| Value  | `{"engine": "mysql","username": "user1","password": "i29wwX!%9wFV","host": "my-database-endpoint.us-east-1.rds.amazonaws.com","dbname": "myDatabase","port": "3306"`} |
| Tags   | `copilot-application=demo`, `copilot-environment=test` |


Manifest を次の様に変更します。
```yaml
secrets:
  # (推奨) オプション 1. 名前を使ってシークレットを参照します。
  DB:
    secretsmanager: 'demo/test/mysql'
  # JSON blob 内の特定のキーを参照できます。
  DB_PASSWORD:
    secretsmanager: 'demo/test/mysql:password::'
  # 事前に定義された環境変数を利用して、Manifest を簡潔に保つ事ができます。
  DB_PASSWORD:
    secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql:password::'

  # オプション 2. 別の方法として、ARN によってシークレットを指定することができます。
  DB: "'arn:aws:secretsmanager:us-west-2:111122223333:secret:demo/test/mysql-Yi6mvL'"
```
