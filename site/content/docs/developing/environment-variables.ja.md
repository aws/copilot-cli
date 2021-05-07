# 環境変数

環境変数は Service が実行されている Environment に応じてサービスから利用可能な変数です。Service は自身で定義することなくそれらの変数を参照できます。環境変数は特定の Environment に固有のデータを Service へ渡したい場合に便利です。例として、テスト用と本番用で接続するデータベースの名前を切り替える場合などです。

環境変数へのアクセス方法は通常、利用しているプログラミング言語によって決まります。ここではいくつかのプログラミング言語で `DATABASE_NAME` という環境変数を取得する例を示します。

__Go__
```go
dbName := os.Getenv("DATABASE_NAME")
```

__Javascript__
```javascript
var dbName = process.env.DATABASE_NAME;
```

__Python__
```python
database_name = os.getenv('DATABASE_NAME')
```

## デフォルト環境変数とは
デフォルトで、AWS Copilot CLI はサービスが利用できるいくつかの環境変数を提供します。

* `COPILOT_APPLICATION_NAME` - この Service を実行している Application 名 
* `COPILOT_ENVIRONMENT_NAME` - Service　が実行されている Environment 名(例: test、prod)
* `COPILOT_SERVICE_NAME` - 現在の Service 名
* `COPILOT_LB_DNS` - (存在する場合)ロードバランサー名。例: _kudos-Publi-MC2WNHAIOAVS-588300247.us-west-2.elb.amazonaws.com_ 注: カスタムドメイン名を利用している場合でも、この値は ロードバランサーの DNS 名を保持します
* `COPILOT_SERVICE_DISCOVERY_ENDPOINT` - サービス検出を介して、Environment の中で他の Service と通信するために Service 名の後に追加されるエンドポイント。値は `{app name}.local` となります。サービスディスカバリについてのより詳しい情報は[サービス検出のガイド](../developing/service-discovery.md) を参照してください

## 環境変数を追加する方法
環境変数を追加するのは簡単です。[Manifest](../manifest/overview.md) の `variables` セクションに直接追加できます。 下記のスニペットでは、`LOG_LEVEL` という変数を `debug` という値で Service に渡しています。

```yaml
# copilot/{service name}/manifest.yml の一部
variables:                    
  LOG_LEVEL: debug
```
Environment に応じて、特定の環境変数の値を渡すこともできます。上記と同じ例で、ログレベルを設定し、production の Environment の時だけ値を `info` に書き換えてみます。Manifest の変更は、それをデプロイした時に反映されるので、ローカルでの変更は安全です。

```yaml
# copilot/{service name}/manifest.yml の一部
variables:                    
  LOG_LEVEL: debug

environments:
  production:
    variables:
      LOG_LEVEL: info
```
ここでは、Manifest を編集して、環境変数を Application に追加する方法の簡単なガイドを紹介しています。👇

![Editing the manifest to add env vars](https://raw.githubusercontent.com/kohidave/ecs-cliv2-demos/master/env-vars-edit.svg?sanitize=true)

## DynamoDB テーブルやS3 バケット、RDS データベースなどの名前を確認する方法

Copilot CLI を使って、DynamoDB テーブルや S3 バケット、データベースなどの追加の AWS リソースをプロビジョニングする場合、出力の値は環境変数として、Application に渡されます。より詳しい情報は、[AWS リソースを追加する](../developing/additional-aws-resources.md)を確認してください。
