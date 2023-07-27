<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
`network` セクションは VPC 内の AWS リソースに接続するための設定です。

<span class="parent-field">network.</span><a id="network-connect" href="#network-connect" class="field">`connect`</a> <span class="type">Bool or Map</span>    
Service に対し [Service Connect](../developing/svc-to-svc-communication.ja.md#service-connect) を有効にします。Service 間のトラフィックを負荷分散し、より弾力的にします。デフォルトは `false` です。

Map として利用すると、Service で利用するエイリアスを指定出来ます。エイリアスは Environment 内でユニークである必要があります。

<span class="parent-field">network.connect.</span><a id="network-connect-alias" href="#network-connect-alias" class="field">`alias`</a> <span class="type">String</span>  
Service Connect 経由で公開する Service のカスタム DNS　名です。デフォルトは Service 名です。

{% include 'network-common.ja.md' %}