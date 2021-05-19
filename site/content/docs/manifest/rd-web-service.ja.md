以下は `'Request-Driven Web Service'` Manifest で利用できるすべてのプロパティのリストです。[Copilot Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

???+ note "frontend Service のサンプル Manifest"

    ```yaml
    # Service 名はロググループや App Runner サービスなどのリソースの命名に利用されます。
    name: frontend
    # 実行する Service の「アーキテクチャ」
    type: Request-Driven Web Service

    http:
      healthcheck:
        path: '/_healthcheck'
        healthy_threshold: 3
        unhealthy_threshold: 5
        interval: 10s
        timeout: 5s

    # コンテナと Service の構成
    image:
      build: ./frontend/Dockerfile
      port: 80

    cpu: 1024
    memory: 2048

    variables:
      LOG_LEVEL: info
    
    tags:
      owner: frontend-team

    environments:
      test:
        variables:
          LOG_LEVEL: debug
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Service のアーキテクチャタイプ。 [Load Balanced Web Service](../concepts/services.ja.md#request-driven-web-service) は、AWS App Runner にデプロイされる、インターネットに公開するための Service です。

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
http セクションは Service と AWS App Runner のマネージドロードバランサの連携に関するパラメーターを含みます。

<span class="parent-field">http.</span><a id="http-healthcheck" href="#http-healthcheck" class="field">`healthcheck`</a> <span class="type">String or Map</span>  
文字列を指定した場合、Copilot は、ターゲットグループからのヘルスチェックリクエストを処理するためにコンテナが公開しているパスと解釈します。デフォルトは "/" です。
```yaml
http:
  healthcheck: '/'
```
あるいは以下のように Map によるヘルスチェックも指定可能です。
```yaml
http:
  healthcheck:
    path: '/'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
```

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-healthy-threshold" href="#http-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
unhealthy なターゲットを healthy とみなすために必要な、連続したヘルスチェックの成功回数を指定します。デフォルト値は 3 で、設定可能な範囲は、1 〜 20 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
ターゲットが unhealthy であると判断するまでに必要な、連続したヘルスチェックの失敗回数を指定します。デフォルト値は 3 で、設定可能な範囲は、1 〜 20 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
個々のターゲットへのヘルスチェックを行う際の、おおよその間隔を秒単位で指定します。デフォルト値は 5 秒で、設定可能な範囲は、1 〜 20 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
ターゲットからの応答がない場合、ヘルスチェックが失敗したとみなすまでの時間を秒単位で指定します。デフォルト値は 2 秒で、設定可能な範囲は、1 〜 20 です。

{% include 'image-config.md' %}

<div class="separator"></div>  

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
Service のインスタンスに割り当てる CPU ユニット数。指定可能な値については [AWS App Runner ドキュメント](https://docs.aws.amazon.com/ja_jp/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Cpu)をご覧ください。

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
タスクに割り当てるメモリ量（MiB）。指定可能な値については [AWS App Runner ドキュメント](https://docs.aws.amazon.com/ja_jp/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Memory)をご覧ください。

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Copilot は Service 名などを常に環境変数としてインスタンスに対して渡します。本フィールドではそれら以外に追加で渡したい環境変数をキーバーリューのペアで指定します。

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
`environments` セクションでは、Manifest 内の任意の設定値を Environment ごとにオーバーライドできます。上部記載の Manifest 例では test Environment における `LOG_LEVEL` 環境変数の値をオーバーライドしています。
