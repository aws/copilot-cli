以下は `'Request-Driven Web Service'` Manifest で利用できるすべてのプロパティのリストです。[Copilot Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

???+ note "AWS App Runner のサンプル Manifest"

    === "Public"

        ```yaml
        # https://web.example.com からアクセス可能な Web サービスをデプロイします。
        name: frontend
        type: Request-Driven Web Service
    
        http:
          healthcheck: '/_healthcheck'
          alias: web.example.com
    
        image:
          build: ./frontend/Dockerfile
          port: 80
        cpu: 1024
        memory: 2048

        variables:
          LOG_LEVEL: info
        tags:
          owner: frontend
        observability:
          tracing: awsxray
        secrets:
          GITHUB_TOKEN: GITHUB_TOKEN
          DB_SECRET:
            secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql'
    
        environments:
          test:
            variables:
              LOG_LEVEL: debug
        ```

    === "Connected to the environment VPC"

        ```yaml
        # すべての Egress トラフィックは、Environment の VPCを経由してルーティングされます。
        name: frontend
        type: Request-Driven Web Service

        image:
          build: ./frontend/Dockerfile
          port: 8080
        cpu: 1024
        memory: 2048

        network:
          vpc:
            placement: private
        ```

    === "Event-driven"

        ```yaml
        # https://aws.github.io/copilot-cli/docs/developing/publish-subscribe/ を参照してください。
        name: refunds
        type: Request-Driven Web Service

        image:
          build: ./refunds/Dockerfile
          port: 8080

        http:
          alias: refunds.example.com
        cpu: 1024
        memory: 2048

        publish:
          topics:
            - name: 'refunds'
            - name: 'orders'
              fifo: true
        ```



<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Service のアーキテクチャタイプ。 [Load Balanced Web Service](../concepts/services.ja.md#request-driven-web-service) は、AWS App Runner にデプロイされる、インターネットに公開するための Service です。

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
http セクションは、マネージドロードバランサの連携に関するパラメーターを含みます。

<span class="parent-field">http.</span><a id="http-private" href="#http-private" class="field">`private`</a> <span class="type">Bool or Map</span>
受信トラフィックを Envrionment のみに制限します。デフォルトは false です。

<span class="parent-field">http.private</span><a id="http-private-endpoint" href="#http-private-endpoint" class="field">`endpoint`</a> <span class="type">String</span>
App Runner に対する既存の VPC エンドポイントの ID です。
```yaml
http:
  private:
    endpoint: vpce-12345
```

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

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-path" href="#http-healthcheck-path" class="field">`path`</a> <span class="type">String</span>  
ヘルスチェック送信先。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-healthy-threshold" href="#http-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
unhealthy なターゲットを healthy とみなすために必要な、連続したヘルスチェックの成功回数を指定します。デフォルト値は 3 で、設定可能な範囲は、1 〜 20 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
ターゲットが unhealthy であると判断するまでに必要な、連続したヘルスチェックの失敗回数を指定します。デフォルト値は 3 で、設定可能な範囲は、1 〜 20 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
個々のターゲットへのヘルスチェックを行う際の、おおよその間隔を秒単位で指定します。デフォルト値は 5 秒で、設定可能な範囲は、1 〜 20 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
ターゲットからの応答がない場合、ヘルスチェックが失敗したとみなすまでの時間を秒単位で指定します。デフォルト値は 2 秒で、設定可能な範囲は、1 〜 20 です。

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String</span>  
Request-Driven Web Service にフレンドリーなドメイン名を割り当てます。詳しくは [developing/domain](../developing/domain.ja.md##request-driven-web-service) をご覧ください。

<div class="separator"></div>

<a id="image" href="#image" class="field">`image`</a> <span class="type">Map</span>  
image セクションは、Docker ビルドに関する設定や公開するポートについてのパラメータを含みます。

<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span>  
このフィールドに String（文字列）を指定した場合、Copilot はそれを Dockerfile の場所を示すパスと解釈します。その際、指定したパスのディレクトリ部が Docker のビルドコンテキストであると仮定します。以下は build フィールドに文字列を指定する例です。
```yaml
image:
  build: path/to/dockerfile
```
これにより、イメージビルドの際に次のようなコマンドが実行されることになります: `$ docker build --file path/to/dockerfile path/to`

build フィールドには Map も利用できます。
```yaml
image:
  build:
    dockerfile: path/to/dockerfile
    context: context/dir
    target: build-stage
    cache_from:
      - image:tag
    args:
      key: value
```
この例は、Copilot は Docker ビルドコンテキストに context フィールドの値が示すディレクトリを利用し、args 以下のキーバリューのペアをイメージビルド時の --build-args 引数として渡します。上記例と同等の docker build コマンドは次のようになります:  
`$ docker build --file path/to/dockerfile --target build-stage --cache-from image:tag --build-arg key=value context/dir`.

Copilot はあなたの意図を理解するために最善を尽くしますので、記述する情報の省略も可能です。例えば、`context` は指定しているが `dockerfile` は未指定の場合、Copilot は Dockerfile が "Dockerfile" という名前で存在すると仮定しつつ、docker コマンドを `context` ディレクトリ以下で実行します。逆に `dockerfile` は指定しているが `context` が未指定の場合は、Copilot はあなたが `dockerfile` で指定されたディレクトリをビルドコンテキストディレクトリとして利用したいのだと仮定します。

すべてのパスはワークスペースのルートディレクトリからの相対パスと解釈されます。

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span>  
Dockerfile からコンテナイメージをビルドする代わりに、既存のコンテナイメージ名の指定も可能です。[`image.build`](#image-build) との同時利用はできません。

!!! note
    現時点では [Amazon ECR Public](https://docs.aws.amazon.com/ja_jp/AmazonECR/latest/public/public-repositories.html) に格納されたコンテナイメージが利用可能です。

<span class="parent-field">image.</span><a id="image-port" href="#image-port" class="field">`port`</a> <span class="type">Integer</span>  
公開するポート番号。Dockerfile 内に `EXPOSE` インストラクションが記述されている場合、Copilot はそれをパースした値をここに挿入します。

<div class="separator"></div>  

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
Service のインスタンスに割り当てる CPU ユニット数。指定可能な値については [AWS App Runner ドキュメント](https://docs.aws.amazon.com/ja_jp/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Cpu)をご覧ください。

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
タスクに割り当てるメモリ量（MiB）。指定可能な値については [AWS App Runner ドキュメント](https://docs.aws.amazon.com/ja_jp/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Memory)をご覧ください。

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
`network` セクションには、Environment の VPC 内の AWS リソースに Service を接続するためのパラメータが含まれています。Service を VPC に接続することで、[サービスディスカバリ](../developing/svc-to-svc-communication.ja.md#service-discovery)を使用して Environment 内の他の Service と通信したり、[`storage init`](../commands/storage-init.ja.md)で Amazon Aurora などの VPC 内のデータベースに接続することができます。

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>    
Service からの Egress トラフィックをルーティングする VPC 内のサブネットを指定します。

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
この項目において現在有効なオプションは `'private'` のみです。もし、Service が VPC に接続されないことを期待する場合は、`network` セクションを削除してください。

この項目が 'private' の場合、App Runner サービスは VPC のプライベートサブネットを経由して Egress トラフィックをルーティングします。
Copilot で生成された VPC を使用する場合、Copilot はインターネット接続用の NAT Gateway を Environment に自動的に追加します。 ([VPC の料金](https://aws.amazon.com/jp/vpc/pricing/)をご覧ください。) また、`copilot env init` を実行する際に、NAT ゲートウェイを持つ既存の VPC や、分離されたワークロードのための VPC エンドポイントをインポートすることも可能です。詳しくは、[Environment のリソースをカスタマイズする](../developing/custom-environment-resources.ja.md)をご覧ください。

{% include 'observability.ja.md' %}

<div class="separator"></div>

<a id="command" href="#command" class="field">`command`</a> <span class="type">String</span>
任意項目。コンテナイメージのデフォルトコマンドをオーバーライドします。

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Copilot は Service 名などを常に環境変数としてインスタンスに対して渡します。本フィールドではそれら以外に追加で渡したい環境変数をキー・値のペアで指定します。

{% include 'secrets.ja.md' %}

{% include 'publish.ja.md' %}

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`tags`</a> <span class="type">Map</span>  
AWS App Runner リソースとして渡される AWS タグを表すキー・値ペアです。

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">String</span>
既存のオートスケーリング設定の名前を指定します。
```yaml
count: high-availability/3
```
<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
`environments` セクションでは、Manifest 内の任意の設定値を Environment ごとにオーバーライドできます。上部記載の Manifest 例では test Environment における `LOG_LEVEL` 環境変数の値をオーバーライドしています。
