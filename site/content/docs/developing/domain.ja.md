# ドメイン
[Application](../concepts/applications.en.md#追加のアプリケーション設定)で説明したように、 `copilot app init` を実行するときに Application のドメイン名を設定できます。 [Load Balanced Web Service](../concepts/services.ja.md#load-balanced-web-service) をデプロイすると以下のようなドメイン名を使ってアクセスできるようになります。

```
${SvcName}.${EnvName}.${AppName}.${DomainName}
```

具体的には例えば以下のようになります。

```
https:kudo.test.coolapp.example.aws
```

## Service にエイリアスを設定する方法
Copilot が Service につけるデフォルトのドメイン名を使いたくない場合は、 Service に[エイリアス](https://docs.aws.amazon.com/ja_jp/Route53/latest/DeveloperGuide/resource-record-sets-choosing-alias-non-alias.html)を設定することも簡単にできます。 [Manifest](../manifest/overview.ja.md) のエイリアスセクションに直接指定できます。以下のスニペットは Service にエイリアスを設定する例です。

``` yaml
# copilot/{service name}/manifest.yml のなかで
http:
  path: '/'
  alias: example.aws
```

!!!info
    現在 Application を作成するときに指定したドメイン配下でしかエイリアスを使うことができません。将来的にこの機能をよりパワフルにする予定で、証明書をインポートし任意のエイリアスを使えるようにする予定です。

## 裏で何がおきているか
裏側では Copilot は

* Application を作成したアカウント内で `${AppName}.${DomainName}` というサブドメイン用のホストゾーンを作成し
* Environment があるアカウント内で `${EnvName}.${AppName}.${DomainName}` という新しい Environment 用のサブドメインのために別のホストゾーンを作成し
* Environment 用のサブドメインに使う ACM 証明書の作成と検証を行い
* HTTPS リスナーと証明書を関連づけて HTTP のトラフィックを HTTPS にリダイレクトし
* エイリアス用でオプションの A レコードを作成しています。
