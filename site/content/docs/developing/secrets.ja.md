# シークレット

シークレットは、OAuth トークン、シークレットキー、API キーなどの機密情報です。これらの情報はアプリケーションコードでは必要ですが、ソースコードにコミットするべきではありません。AWS Copilot CLI では、シークレットは環境変数として渡されますが、その機密性のため扱いが異なります (詳細は[環境変数を使った開発](../developing/environment-variables.ja.md)を参照して下さい) 。

## シークレットの追加方法

現在、シークレットを追加するには、シークレットを SecureString として [AWS Systems Manager パラメータストア](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/systems-manager-parameter-store.html) (SSM) に保存する必要があります。そして、SSM パラメータへの参照を [Manifest](../manifest/overview.ja.md) に追加します。

[`copilot secret init`](../commands/secret-init.ja.md) コマンドを利用することで簡単にシークレットを作成できます！シークレットを作成すると、Copilot はどのような名前で作られたかを教えてくれます。その名前を Service や Job の Manifest に記述しましょう。

### あるいは...

Copilot の外で作成したシークレットを持ち込みたい場合、そのシークレットに次の２つのタグを設定することを忘れないようにしてください - `copilot-application: <このシークレットを利用したい Copilot Application 名>` と `copilot-environment: <このシークレットを利用したい Copilot Environment 名>.`

上記の `copilot-application` と `copilot-environment` タグは、Copilot が持ち込みシークレットへのアクセスを適切に制御するために必要となります。

`GH_WEBHOOK_SECRET` という名前で値に `secretvalue1234` を持つ（適切にタグが設定された）SSM パラメータがあると仮定しましょう。このシークレットを Manifest ファイルから参照するには、次のような内容を Manifest に記述することになります。

```yaml
secrets:                      
  GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET  
```

更新されたマニフェストをデプロイすると、Service や Job は環境変数 `GITHUB_WEBHOOK_SECRET` にアクセスできるようになります。この環境変数には、SSM パラメータ `GH_WEBHOOK_SECRET` の値である `secretvalue1234` が格納されます。

これが機能するのは、ECS エージェントがタスクの開始時に SSM パラメータを解決し、環境変数を設定してくれるためです。

!!! attention
    Request-Driven Web Service はシークレットの利用をサポートしていません。
