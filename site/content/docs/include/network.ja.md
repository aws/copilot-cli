<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
`network` セクションは VPC 内の AWS リソースに接続するための設定です。

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>    
タスクを配置するサブネットとアタッチされるセキュリティグループの設定です。

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
`public` あるいは `private` のどちらかを指定します。デフォルトではタスクはパブリックサブネットに配置されます。

!!! info
    Copilot が生成した VPC を利用して `private` サブネットにタスクを配置する場合、Copilot は Environment にインターネット接続用の NAT ゲートウェイを作成します。(価格は[こちら](https://aws.amazon.com/vpc/pricing/)。)
    `copilot env init` コマンドで既存の VPC をインポートして利用することや、分離されたワークロード用に VPC エンドポイントが構成された VPC を構成ができます。詳細は、[custom environment resources](../developing/custom-environment-resources.ja.md)を確認してください。

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>  
Copilot は、Service が Environment 内の他の Service と通信できるようにするためのセキュリティグループを常にタスクに対して設定します。本フィールドでは、同セキュリティグループ以外に追加で紐づけたい １ つ以上のセキュリティグループ ID を指定します。
