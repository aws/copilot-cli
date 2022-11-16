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
    port: 8080
    success_codes: '200'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
    grace_period: 60s
```

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-path" href="#http-healthcheck-path" class="field">`path`</a> <span class="type">String</span>  
ヘルスチェックリクエスト送信先。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-port" href="#http-healthcheck-port" class="field">`port`</a> <span class="type">Integer</span>
ヘルスチェックリクエストを送信するポート。デフォルト値は、[`image.port`](./#image-port) です。[`http.target_container`](./#http-target-container)で公開ポートが設定されている場合、公開しているポートが設定されます。  
ポートが `443` で公開されている場合、 ヘルスチェックは自動で HTTPS に設定されます。

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
