# ドメイン

!!!attention
    現時点では、 `copilot app init` を実行した時のみに Route 53　ドメインが関連づけられます。
    Application のドメインをアップデートしたい場合 ([#3045](https://github.com/aws/copilot-cli/issues/3045)) 、
    `--domain` オプションをつけて、重複した Application を初期化する必要があります。その後、 `copilot app delete` を実行し、
    古い Application を削除します。

## Load Balanced Web Service
[Application](../concepts/applications.en.md#追加のアプリケーション設定)で説明したように、 `copilot app init` を実行するときに Application のドメイン名を設定できます。 [Load Balanced Web Service](../concepts/services.ja.md#load-balanced-web-service) をデプロイすると以下のようなドメイン名を使ってアクセスできるようになります。

```
${SvcName}.${EnvName}.${AppName}.${DomainName}
```

上記は、Application Load Balancer を利用している場合です。

```
${SvcName}-nlb.${EnvName}.${AppName}.${DomainName}
```

これは、Network Load Balancer を利用している場合です。

具体的には、 `https://kudo.test.coolapp.example.aws` や `kudo-nlb.test.coolapp.example.aws:443` となります。


現在、エイリアスは Application 作成時に指定したドメインの下でのみ使用できます。[サブドメインの責任の Route 53 への委任](https://docs.aws.amazon.com/ja_jp/Route53/latest/DeveloperGuide/CreatingNewSubdomain.html#UpdateDNSParentDomain)により、指定したエイリアスは、以下の 3 つのホストゾーンの 1 つでなければなりません。

- root: `${DomainName}`
- app: `${AppName}.${DomainName}`
- env: `${EnvName}.${AppName}.${DomainName}`

将来的には証明書をインポートしたり、任意のエイリアスを使用できるようにして、この機能をより強力なものにする予定です！
    
## Service にエイリアスを設定する方法
Copilot が Service につけるデフォルトのドメイン名を使いたくない場合は、 Service に[エイリアス](https://docs.aws.amazon.com/ja_jp/Route53/latest/DeveloperGuide/resource-record-sets-choosing-alias-non-alias.html)を設定することも簡単にできます。 [Manifest](../manifest/overview.ja.md) のエイリアスセクションに直接指定できます。
以下のスニペットは Service にエイリアスを設定する例です。

``` yaml
# copilot/{service name}/manifest.yml のなかで
http:
  path: '/'
  alias: example.aws
```


## 裏で何がおきているか
裏側では Copilot は

* Application を作成したアカウント内で `${AppName}.${DomainName}` というサブドメイン用のホストゾーンを作成し
* Environment があるアカウント内で `${EnvName}.${AppName}.${DomainName}` という新しい Environment 用のサブドメインのために別のホストゾーンを作成し
* Environment 用のサブドメインに使う ACM 証明書の作成と検証し
* ACM 証明書を
    - エイリアスが Application Load Balancer　(`http.alias`) として利用されている場合、HTTPS リスナー と関連づけて HTTP のトラフィックを HTTPS にリダイレクトし、
    - エリアスが `nlb.alias` として利用されていて、 TLS ターミーネーションが有効な場合、 Network Load Balancer の TLS リスナー
* エイリアス用でオプションの A レコードを作成しています。

## デモ
[![AWS Copilot CLI v1.8.0 Release Highlights](https://img.youtube.com/vi/Oyr-n59mVjI/0.jpg)](https://www.youtube.com/embed/Oyr-n59mVjI)

## Request-Driven Web Service
Request-Driven Web Service に[カスタムドメイン](https://docs.aws.amazon.com/ja_jp/apprunner/latest/dg/manage-custom-domains.html)を追加もできます。Load Balanced Web Service と同様に、Manifest の [`alias`](../manifest/rd-web-service.en.md#http-alias) フィールドを変更することで追加できます。
```yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
  alias: web.example.aws
```

Load Balanced Web Service 同様、Request-Driven Web Service がドメインを使用するためには Application がドメイン (例：example.aws) と関連付けられている必要があります。

!!!info
    現時点では、`web.example.aws` のような 1 レベルのサブドメインのみをサポートしています。

    Environment レベルのドメイン (例：`web.${envName}.${appName}.example.aws`) や、Application レベルのドメイン (例：`web.${appName}.example.aws`)、
    ルートドメイン (例：`example.aws`) はまだサポートされていません。これは、サブドメインが Application 名と衝突してはいけないということでもあります。

Copilot は内部的には以下のような処理を行なっています。

* ドメインを app runner service に関連付けます。
* ルートドメインのホストゾーンにドメインレコードと検証レコードを作成します。
