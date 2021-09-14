<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>   
http セクションは Service と Application Load Balancer の連携に関するパラメーターを含みます。

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>  
このパスに対するリクエストが Service に転送されます。各 [Load Balanced Web Service](../concepts/services.ja.md#load-balanced-web-service) は、ユニークなパスでリッスンする必要があります。

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
    success_codes: '200'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
    grace_period: 60s
```

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-path" href="#http-healthcheck-path" class="field">`path`</a> <span class="type">String</span>  
ヘルスチェックリクエスト送信先。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-success-codes" href="#http-healthcheck-success-codes" class="field">`success_codes`</a> <span class="type">String</span>  
healthy なターゲットがヘルスチェックに対して返す HTTP ステータスコードを指定します。200 から 499 の範囲で指定可能です。また、"200,202" のように複数の値を指定することや "200-299" のような値の範囲指定も可能です。デフォルト値は 200 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-healthy-threshold" href="#http-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
unhealthy なターゲットを healthy とみなすために必要な、連続したヘルスチェックの成功回数を指定します。デフォルト値は 5 で、設定可能な範囲は、2 〜 10 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
ターゲットが unhealthy であると判断するまでに必要な、連続したヘルスチェックの失敗回数を指定します。デフォルト値は 2 で、設定可能な範囲は、2 〜 10 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
個々のターゲットへのヘルスチェックを行う際の、おおよその間隔を秒単位で指定します。デフォルト値は 30 秒で、設定可能な範囲は、5 〜 300 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
ターゲットからの応答がない場合、ヘルスチェックが失敗したとみなすまでの時間を秒単位で指定します。デフォルト値は 5 秒で、設定可能な範囲は、5 〜 300 です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-grace-period" href="#http-healthcheck-grace-period" class="field">`grace_period`</a> <span class="type">Duration</span>  
コンテナ起動時にターゲットグループのヘルスチェックが失敗した場合の、それを無視する時間を指定します。デフォルトは 60 秒です。これは、healthy であることを担保しながら着信を待機するまでに時間がかかるコンテナのデプロイ時の問題を修正したり、迅速な起動が保証されているコンテナのデプロイを高速化したりするのに役立ちます。

<span class="parent-field">http.</span><a id="http-deregistration-delay" href="#http-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>  
登録解除時にターゲットがクライアントとの接続を閉じるのを待つ時間を指定します。デフォルトでは 60 秒です。この値を大きくするとターゲットが安全に接続を閉じるための時間を確保できますが、新バージョンのデプロイに必要となる時間が長くなります。範囲は 0 〜 3600 です。

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>  
サイドカーコンテナを指定することで、Service のメインコンテナの代わりにサイドカーでロードバランサからのリクエストを受け取れます。

<span class="parent-field">http.</span><a id="http-stickiness" href="#http-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
スティッキーセッションの有効化、あるいは無効化を指定します。

<span class="parent-field">http.</span><a id="http-allowed-source-ips" href="#http-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
Service へのアクセスを許可する CIDR IP アドレスのリストを指定します。
```yaml
http:
  allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
```

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String or Array of Strings</span>  
サービスの HTTPS ドメインエイリアス
```yaml
# 文字列で指定する場合
http:
  alias: example.com
# 別の方法として、文字列配列の場合
http:
  alias: ["example.com", "v1.example.com"]
```
