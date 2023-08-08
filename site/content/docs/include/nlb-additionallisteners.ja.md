??? note "nlb.additional_listeners Map"
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-port" href="#nlb-additional-listeners-port" class="field">`port`</a> <span class="type">String</span>  
    必須項目。Network Load Balancer が待ち受ける為の追加ポートとプロトコル。
    
    使用可能なプロトコルには `tcp` 、`udp` と `tls` です。プロトコルが指定されていない場合、デフォルト値として `tcp` が使用されます。
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-healthcheck" href="#nlb-additional-listeners-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>
    Network Load Balancer の追加リスナーに対するヘルスチェック設定を指定します。
    ```yaml
    nlb:
      additional_listeners:
        - healthcheck:
            port: 80
            healthy_threshold: 3
            unhealthy_threshold: 2
            interval: 15s
            timeout: 10s
    ```
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-port" href="#nlb-additional-listeners-healthcheck-port" class="field">`port`</a> <span class="type">String</span>  
    ヘルスチェックリクエストが送信されるポート。コンテナターゲットポートと異なるポートでヘルスチェックが実行される場合に指定します。
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-healthy-threshold" href="#nlb-additional-listeners-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
    unhealthy なターゲットを healthy とみなすために必要な、連続したヘルスチェックの成功回数を指定します。デフォルト値は 3 で、設定可能な範囲は、2-10 です。
    

    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-unhealthy-threshold" href="#nlb-additional-listeners-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
    ターゲットが unhealthy であると判断するまでに必要な、連続したヘルスチェックの失敗回数を指定します。デフォルト値は 3 で、設定可能な範囲は、2-10 です。
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-interval" href="#nlb-additional-listeners-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
    個々のターゲットへのヘルスチェックを行う際の、おおよその間隔を秒単位で指定します。 10 秒または 30 秒が設定可能です。デフォルト値は 30 秒です。
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-timeout" href="#nlb-additional-listeners-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
    ターゲットからの応答がない場合、ヘルスチェックが失敗したとみなすまでの時間を秒単位で指定します。デフォルト値は 10 秒です。
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-target-container" href="#nlb-additional-listeners-target-container" class="field">`target_container`</a> <span class="type">String</span>  
    サービスコンテナの代わりとなるサイドカーコンテナ。
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-target-port" href="#nlb-additional-listeners-target-port" class="field">`target_port`</a> <span class="type">Integer</span>  
    トラフィックを受信するコンテナポート。コンテナポートが `nlb.port`、リスナーポートと異なる場合にこのフィールドを指定します。
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-ssl-policy" href="#nlb-additional-listeners-ssl-policy" class="field">`ssl_policy`</a> <span class="type">String</span> 
  　サポートするプロコルと暗号を定義するセキュリティポリシーです。詳細については、[こちらのドキュメント](https://docs.aws.amazon.com/ja_jp/elasticloadbalancing/latest/network/create-tls-listener.html#describe-ssl-policies)を確認してください。
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-stickiness" href="#nlb-additional-listeners-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
    スティッキーセッションの有効化、あるいは無効化を指定します。