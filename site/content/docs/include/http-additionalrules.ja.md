<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-path" href="#http-additional-rules-path" class="field">`path`</a> <span class="type">String</span> 
    指定したパスに対するリクエストが、Service に転送されます。各リスナールールはユニークなパスで
    公開している必要があります。 

{% include 'http-additionalrules-healthcheck.ja.md' %}
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-deregistration-delay" href="#http-additional-rules-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span> 
    登録解除時に、ターゲットがコネクションをドレイニングするのを待つ時間です。デフォルト値は 60 秒です。大きな値に設定すると、安全にコネクションをドレイニングするのに長い時間を使える様になりますが、新しいデプロイに必要な時間が増加します。設定可能な範囲は、0 秒 - 3600 秒です。
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-target-container" href="#http-additional-rules-target-container" class="field">`target_container`</a> <span class="type">String</span>  
    メインのサービスコンテナの代わりにリクエストがルーティングされるサイドカーコンテナ。
    ターゲットコンテナのポートが 443 に設定されている場合、 ロードバランサーがターゲットコンテナにインストールされた証明書を使用して、Fargate タスクとの TLS 接続するために、プロトコルは `HTTPS` に設定されます。
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-target-port" href="#http-additional-rules-target-port" class="field">`target_port`</a> <span class="type">String</span>  
    トラフィックを受信するコンテナポート。 メインコンテナの `image.port` やサイドカーコンテナの `sidecar.port` と異なるコンテナポートの場合、このフィールドを指定します。
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-stickiness" href="#http-additional-rules-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
    スティッキーセッションの有効化、あるいは無効化を指定します。
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-allowed-source-ips" href="#http-additional-rules-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
    Service へアクセスを許可する CIDR IP アドレスです。
    ```yaml
    http:
      additional_rules:
        - allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
    ```
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-alias" href="#http-additional-rules-alias" class="field">`alias`</a> <span class="type">String or Array of Strings or Array of Maps</span>
    Service の HTTPS ドメインエイリアスです。
    ```yaml
    # String version.
    http:
      additional_rules:
        - alias: example.com
    # Alternatively, as an array of strings.
    http:
      additional_rules:
        - alias: ["example.com", "v1.example.com"]
    # Alternatively, as an array of maps.
    http:
      additional_rules:
        - alias:
            - name: example.com
              hosted_zone: Z0873220N255IR3MTNR4
            - name: v1.example.com
              hosted_zone: AN0THE9H05TED20NEID
    ```
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-hosted-zone" href="#http-additional-rules-hosted-zone" class="field">`hosted_zone`</a> <span class="type">String</span>
    既存のホステッドゾーンの ID。 `http.alias` および `http.additional_rules.alias` と共にのみ使用できます。証明書をインポートした Environment がある場合、ロードバランサーの作成後に、Copilot が A レコードを挿入するホストゾーンを指定できます。
    ```yaml
    http:
      additional_rules:
        - alias: example.com
          hosted_zone: Z0873220N255IR3MTNR4
    # Also see http.alias array of maps example, above.
    ```
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-redirect-to-https" href="#http-additional-rules-redirect-to-https" class="field">`redirect_to_https`</a> <span class="type">Boolean</span>  
    Application Load Balancer で HTTP から HTTPS に自動的にリダイレクトします。デフォルトは `true` です。 

<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-version" href="#http-additional-rules-version" class="field">`version`</a> <span class="type">String</span> 
    HTTP(S) のプロトコルバージョン。`'grpc'` 、 `'http1'`、または `'http2'` のどれかです。省略した場合、`'http1'` が推定されます。
    gRPC を使用する場合、ドメインがアプリケーションと関連付けられている必要があることに注意してください。