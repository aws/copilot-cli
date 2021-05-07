# シークレット

シークレットは、OAuth トークン、シークレットキー、API キーなどの機密情報です。これらの情報はアプリケーションコードでは必要ですが、ソースコードにコミットするべきではありません。AWS Copilot CLI では、シークレットは環境変数として渡されますが、その機密性のため扱いが異なります (詳細は[環境変数を使った開発](../developing/environment-variables.md)を参照して下さい) 。

## シークレットの追加方法

現在、シークレットを追加するには、シークレットを SecureString として [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) (SSM) に保存する必要があります。そして、SSM パラメータへの参照を [Manifest](../manifest/overview.md) に追加します。

ここでは、`GH_WEBHOOK_SECRET` という名前のシークレットを、`secretvalue1234` という値で保存する例を見てみましょう。

まず、次のようにシークレットを SSM に保存します。

```sh
aws ssm put-parameter --name GH_WEBHOOK_SECRET --value secretvalue1234 --type SecureString\
  --tags Key=copilot-environment,Value=${ENVIRONMENT_NAME} Key=copilot-application,Value=${APP_NAME}
```

これにより、SSM のパラメータ `GH_WEBHOOK_SECRET` に値 `secretvalue1234` がシークレットとして格納されます。

!!! attention
    Copilot では、このシークレットへのアクセスを制限するために、 `copilot-application` と `copilot-environment` タグが必要です。
    `${ENVIRONMENT_NAME}` と `${APP_NAME}` を、このシークレットへのアクセスを許可したい Copilot Application と Environment に置き換えることが重要です。


次に、この値を渡すために Manifest ファイルを修正します。

```yaml
secrets:                      
  GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET  
```

マニフェストのこの更新をデプロイすると、環境変数 `GITHUB_WEBHOOK_SECRET` にアクセスできるようになります。この環境変数には、SSM パラメータ `GH_WEBHOOK_SECRET` の値である `secretvalue1234` が格納されています。

これが機能するのは、ECS エージェントがタスクの開始時に SSM パラメータを解決し、環境変数を設定してくれるためです。

!!! info
    **この機能はもっと簡単にする予定です！** Application と同じ Environment にシークレットを保存しなければならないという、いくつかの注意点があります。
    将来的に `secrets` コマンドを追加して、どの Environment にいるのかや SSM がどのように動作するのかを気にすることなく、シークレットを追加できるようにしたいと考えています。
