<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
`network` セクションは VPC 内の AWS リソースに接続するための設定です。

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>    
タスクを配置するサブネットとアタッチされるセキュリティグループの設定です。

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
`public` あるいは `private` のどちらかを指定します。デフォルトではタスクはパブリックサブネットに配置されます。

!!! info
    Copilot が生成した VPC を利用して `private` サブネットにタスクを配置する場合、Copilot は Environment にインターネット接続用の NAT ゲートウェイを作成します。(価格は[こちら](https://aws.amazon.com/vpc/pricing/)。)
    あるいは `copilot env init` コマンドで既存の VPC をインポートして利用することや、分離されたワークロード用に VPC エンドポイントが構成された VPC を構成ができます。詳細は、[custom environment resources](../developing/custom-environment-resources.ja.md)を確認してください。

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>
Copilot がタスクに対して自動で設定するセキュリティグループ以外に追加で設定したいセキュリティグループがある場合にそれらの ID を指定します。複数のセキュリティグループ ID を指定可能です。(Copilot が自動設定するセキュリティグループは、同一 Environment 内の Service 間通信を可能にする目的で設定されます。)
