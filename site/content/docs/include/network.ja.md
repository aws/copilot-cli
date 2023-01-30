<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
`network` セクションは VPC 内の AWS リソースに接続するための設定です。

<span class="parent-field">network.</span><a id="network-connect" href="#network-connect" class="field">`connect`</a> <span class="type">Bool or Map</span>    
Service に対し [Service Connect](../developing/svc-to-svc-communication.ja.md#service-connect) を有効にします。Service 間のトラフィックを負荷分散し、より弾力的にします。デフォルトは `false` です。

Map として利用すると、Service で利用するエイリアスを指定出来ます。エイリアスは Environment 内でユニークである必要があります。

<span class="parent-field">network.connect.</span><a id="network-connect-alias" href="#network-connect-alias" class="field">`alias`</a> <span class="type">String</span>  
Service Connect 経由で公開する Service のカスタム DNS　名です。デフォルトは Service 名です。

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>    
タスクを配置するサブネットとアタッチされるセキュリティグループの設定です。

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String or Map</span>  
String として利用する場合、`public` あるいは `private` のどちらかを指定します。デフォルトではタスクはパブリックサブネットに配置されます。

!!! info
    Copilot が生成した VPC を利用して `private` サブネットにタスクを配置する場合、Copilot は Environment にインターネット接続用の NAT ゲートウェイを作成します。(価格は[こちら](https://aws.amazon.com/vpc/pricing/)。)あるいは `copilot env init` コマンドで既存の VPC をインポートして利用することや、分離されたワークロード用に VPC エンドポイントが構成された VPC を構成ができます。詳細は、[custom environment resources](../developing/custom-environment-resources.ja.md)を確認してください。

Map として利用する場合、 Copilot が ECS タスクを起動するサブネットを指定します。例：

```yaml
network:
  vpc:
    placement:
      subnets: ["SubnetID1", "SubnetID2"]
```

<span class="parent-field">network.vpc.placement.</span><a id="network-vpc-placement-subnets" href="#network-vpc-placement-subnets" class="field">`subnets`</a> <span class="type">Array of Strings or Map</span>  
String のリストとする場合、Copilot が ECS タスクを起動するサブネット ID を指定します。

Map の場合、サブネットをフィルタリングするための名前と値のペアを指定します。フィルタは `AND` で結合され、各フィルタの値は `OR` で結合されることに注意してください。例えば、タグセット `org: bi` と `type: public` を持つサブネットと、タグセット `org: bi` と `type: private` を持つサブネットの両方は、以下の方法でマッチングされることになります。

```yaml
network:
  vpc:
    placement:
      subnets:
        from_tags:
          org: bi
          type:
            - public
            - private
```

<span class="parent-field">network.vpc.placement.subnets</span><a id="network-vpc-placement-subnets-from-tags" href="#network-vpc-placement-subnets-from-tags" class="field">`from_tags`</a> <span class="type">Map of String and String or Array of Strings</span>  
Copilot が ECS タスクを起動するサブネットをフィルタリングするためのタグセット。

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings or Map</span>  
タスクに関連する追加のセキュリティグループ ID。
```yaml
network:
  vpc:
    security_groups: [sg-0001, sg-0002]
```
Copilot にはセキュリティグループが含まれており、Environment 内のコンテナ同士が通信できるようになっています。デフォルトのセキュリティグループを無効にするには、`Map` 形式で以下のように指定します。
```yaml
network:
  vpc:
    security_groups:
      deny_default: true
      groups: [sg-0001, sg-0002]
```

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-from-cfn" href="#network-vpc-security-groups-from-cfn" class="field">`from_cfn`</a> <span class="type">String</span>  
[CloudFormation スタックエクスポート](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html)の名称。

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-deny-default" href="#network-vpc-security-groups-deny-default" class="field">`deny_default`</a> <span class="type">Boolean</span>  
Environment 内のすべての Service からの侵入を許可するデフォルトのセキュリティグループを無効化します。

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-groups" href="#network-vpc-security-groups-groups" class="field">`groups`</a> <span class="type">Array of Strings</span>    
タスクに関連する追加のセキュリティグループ ID。

<span class="parent-field">network.vpc.security_groups.groups</span><a id="network-vpc-security-groups-groups-from-cfn" href="#network-vpc-security-groups-groups-from-cfn" class="field">`from_cfn`</a> <span class="type">String</span>  
[CloudFormation スタックエクスポート](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html)の名称。
