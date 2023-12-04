以下は `'Static Site Service'` Manifest で利用できるすべてのプロパティのリストです。

???+ note "Static Site の サンプル Manifest"

    ```yaml
    name: example
    type: Static Site

    http:
      alias: 'example.com'

    files:
      - source: src/someDirectory
        recursive: true
      - source: someFile.html
    
    # 上記で定義された値は Environment によるオーバーライドが可能です。
    # environments:
    #   test:
    #     files:
    #       - source: './blob'
    #         destination: 'assets'
    #         recursive: true
    #         exclude: '*'
    #         reinclude:
    #           - '*.txt'
    #           - '*.png'
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Service のアーキテクチャタイプ。[Static Site](../concepts/services.ja.md#static-site) は、Amazon S3 によってホストされているインターネットに面した Service です。

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
サイトへの受信トラフィックの設定。

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String</span>  
Service の HTTPS ドメインエイリアス。

<span class="parent-field">http.</span><a id="http-certificate" href="#http-certificate" class="field">`certificate`</a> <span class="type">String</span>  
HTTPS トラフィックに利用する証明書の ARN。
CloudFront で ACM 証明書を利用するには `us-east-1` リージョンの証明書をインポートする必要があります。以下は、Manifest の一部の例です。

```yaml
http:
  alias: example.com
  certificate: "arn:aws:acm:us-east-1:1234567890:certificate/e5a6e114-b022-45b1-9339-38fbfd6db3e2"
```

<div class="separator"></div>

<a id="files" href="#files" class="field">`files`</a> <span class="type">Array of Maps</span>  
静的アセットに関連するパラメータ。

<span class="parent-field">files.</span><a id="files-source" href="#files-source" class="field">`source`</a> <span class="type">String</span>  
ワークスペースのルートからの相対パスとして、S3 にアップロードするディレクトリまたはファイルへのパスを指定します。

<span class="parent-field">files.</span><a id="files-recursive" href="#files-recursive" class="field">`recursive`</a> <span class="type">Boolean</span>  
ソースディレクトリを再帰的にアップロードするかどうか。ディレクトリの場合、デフォルトは true です。

<span class="parent-field">files.</span><a id="files-destination" href="#files-destination" class="field">`destination`</a> <span class="type">String</span>  
任意項目。S3 バケット内のファイルに付加されるサブパスを指定します。デフォルトは `.` です。

<span class="parent-field">files.</span><a id="files-exclude" href="#files-exclude" class="field">`exclude`</a> <span class="type">String</span>  
任意項目。アップロードファイルを除外するためのパターンマッチの[フィルター](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3/index.html#use-of-exclude-and-include-filters)。使用可能なシンボルは以下の通りです。  
`*` (全てにマッチする)  
`?` (任意の 1 文字にマッチする)  
`[sequence]` (`sequence` の任意の文字にマッチする)  
`[!sequence]` (`sequence` に含まれない文字にマッチする)  

<span class="parent-field">files.</span><a id="files-reinclude" href="#files-reinclude" class="field">`reinclude`</a> <span class="type">String</span>  
任意項目。[`exclude`](#files-exclude) でアップロードから除外されたファイルを再度インクルードするためのパターンマッチの[フィルター](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3/index.html#use-of-exclude-and-include-filters)。使用可能なシンボルは以下の通りです。  
`*` (全てにマッチする)  
`?` (任意の 1 文字にマッチする)  
`[sequence]` (`sequence` の任意の文字にマッチする)  
`[!sequence]` (`sequence` に含まれない文字にマッチする)  
