<div class="separator"></div>

<a id="nlb" href="#nlb" class="field">`nlb`</a> <span class="type">Map</span>  
nlb セクションは Service を Network Load Balancer と統合するためのパラメーターを含みます。

Network Load Balancerは、`nlb` フィールドを指定した場合のみ有効になります。
Load Balanced Web Service では、Application Load Balancer と Network Load Balancer のいずれかが有効になっている必要があることに注意してください。

<span class="parent-field">nlb.</span><a id="nlb-port" href="#nlb-port" class="field">`port`</a> <span class="type">String</span>  
必須項目。Network Load Balancer がリッスンするポートとプロトコルを指定します。

使用可能なプロトコルは `tcp` 、 `udp` と `tls` です。プロトコルを指定しない場合、デフォルトで `tcp` が使用されます。  
設定例:
```yaml
nlb:
  port: 80
```
`tcp` リクエストをポート 80 で待ち受けるようにするためには、以下のように設定します。  
設定例:
```yaml
nlb:
  port: 80/tcp
```

簡単に TLS 終端を有効にすることができます。  
設定例:
```yaml
nlb:
  port: 443/tls
```

<span class="parent-field">nlb.</span><a id="nlb-healthcheck" href="#nlb-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
Network Load Balancer のヘルスチェックの設定を指定します。
```yaml
nlb:
  healthcheck:
    port: 80
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
```

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-port" href="#nlb-healthcheck-port" class="field">`port`</a> <span class="type">String</span>  
ヘルスチェックのリクエストが送信されるポート。ヘルスチェックが、コンテナターゲットポートとは異なるポートで実行される必要がある場合に指定します。

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-healthy-threshold" href="#nlb-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
unhealthy なターゲットを healthy とみなすために必要な、連続したヘルスチェックの成功回数を指定します。デフォルト値は 3 で、設定可能な範囲は、2 〜 10 です。

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-unhealthy-threshold" href="#nlb-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
ターゲットが unhealthy であると判断するまでに必要な、連続したヘルスチェックの失敗回数を指定します。デフォルト値は 3 で、設定可能な範囲は、2 〜 10 です。

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-grace-period" href="#nlb-healthcheck-grace-period" class="field">`grace_period`</a> <span class="type">Duration</span>  
コンテナ起動時にターゲットグループのヘルスチェックが失敗した場合の、それを無視する時間を指定します。デフォルトは 60 秒です。これは、healthy であることを担保しながら着信を待機するまでに時間がかかるコンテナのデプロイ時の問題を修正したり、迅速な起動が保証されているコンテナのデプロイを高速化したりするのに役立ちます。

!!! info
    この説明を書いている時点では、[ドキュメント](https://docs.aws.amazon.com/ja_jp/elasticloadbalancing/latest/network/target-group-health-checks.html)によると、Network Load Balancer の 'unhealthy threshold' は 'healthy threshold' と同じである必要があるとされています。

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-interval" href="#nlb-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
個々のターゲットへのヘルスチェックを行う際の、おおよその間隔を秒単位で指定します。設定可能な値は 10s (10 秒) または 30s (30 秒) で、デフォルト値は 30s です。

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-timeout" href="#nlb-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
ターゲットからの応答がない場合、ヘルスチェックが失敗したとみなすまでの時間を秒単位で指定します。デフォルト値は 10s (10 秒)です。

<span class="parent-field">nlb.</span><a id="nlb-target-container" href="#nlb-target-container" class="field">`target_container`</a> <span class="type">String</span>  
サイドカーコンテナを指定することで、Service のメインコンテナの代わりにサイドカーでロードバランサからのリクエストを受け取れます。

<span class="parent-field">nlb.</span><a id="nlb-target-port" href="#nlb-target-port" class="field">`target_port`</a> <span class="type">Integer</span>  
トラフィックを受信するコンテナのポート。コンテナポートがリスナーポートの `nlb.port` と異なる場合、このフィールドを指定します。

<span class="parent-field">nlb.</span><a id="nlb-ssl-policy" href="#nlb-ssl-policy" class="field">`ssl_policy`</a> <span class="type">String</span>  
どのようなプロトコルや暗号をサポートするかを定義するセキュリティポリシーです。詳しくは[このドキュメント](https://docs.aws.amazon.com/ja_jp/elasticloadbalancing/latest/network/create-tls-listener.html#describe-ssl-policies)をご覧ください。

<span class="parent-field">nlb.</span><a id="nlb-stickiness" href="#nlb-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
スティッキーセッションの有効化、あるいは無効化を指定します。

<span class="parent-field">nlb.</span><a id="nlb-alias" href="#nlb-alias" class="field">`alias`</a> <span class="type">String or Array of Strings</span>  
Service のドメインエイリアス
```yaml
# 文字列で指定する場合
nlb:
  alias: example.com
# 別の方法として、文字列配列の場合
nlb:
  alias: ["example.com", "v1.example.com"]
```
<span class="parent-field">nlb.</span><a id="nlb-additional-listeners" href="#nlb-additional-listeners" class="field">`additional_listeners`</a> <span class="type">Array of Maps</span>  
複数の NLB リスナーを設定します。

{% include 'nlb-additionallisteners.ja.md' %}