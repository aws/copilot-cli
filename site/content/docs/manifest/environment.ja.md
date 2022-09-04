`'Environment'` Manifest で利用可能なすべてのプロパティのリストです。
Copilot Environment の詳細については、[Environment](../concepts/environments.ja.md) のコンセプトページを参照してください。

???+ note "Environment のサンプル Manifest"

    === "Basic"

        ```yaml
        name: prod
        type: Environment
        observability:
          container_insights: true
        ```

    === "Imported VPC"

        ```yaml
        name: imported
        type: Environment
        network:
          vpc:
            id: 'vpc-12345'
            subnets:
              public:
                - id: 'subnet-11111'
                - id: 'subnet-22222'
              private:
                - id: 'subnet-33333'
                - id: 'subnet-44444'
        ```

    === "Configured VPC"

        ```yaml
        name: qa
        type: Environment
        network:
          vpc:
            cidr: '10.0.0.0/16'
            subnets:
              public:
                - cidr: '10.0.0.0/24'
                  az: 'us-east-2a'
                - cidr: '10.0.1.0/24'
                  az: 'us-east-2b'
              private:
                - cidr: '10.0.3.0/24'
                  az: 'us-east-2a'
                - cidr: '10.0.4.0/24'
                  az: 'us-east-2b'
        ```

    === "With public certificates"

        ```yaml
        name: prod-pdx
        type: Environment
        http:
          public: # 既存の証明書をパブリックロードバランサーに適用します
            certificates:
              - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/13245665-cv8f-adf3-j7gd-adf876af95
        ```

    === "Private"

        ```yaml
        name: onprem
        type: Environment
        network:
          vpc:
            id: 'vpc-12345'
            subnets:
              private:
                - id: 'subnet-11111'
                - id: 'subnet-22222'
                - id: 'subnet-33333'
                - id: 'subnet-44444'
        http:
          private: # 既存の証明書をプライベートロードバランサーに適用します。
            certificates:
              - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/13245665-cv8f-adf3-j7gd-adf876af95
            subnets: ['subnet-11111', 'subnet-22222']
        ```

    === "Content delivery network"

        ```yaml
        name: cloudfront
        type: Environment
        cdn: true
        http:
          public:
            security_groups:
             ingress: 
               restrict_to:
                 cdn: true
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Environment の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
`'Environment'` に設定されている必要があります.

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>  
network セクションには、既存の VPC をインポートするためのパラメータ、または Copilot で生成された VPC を設定するためのパラメータが含まれています。

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>  
vpc セクションには、CIDR 設定やサブネットの設定を行うためのパラメータが含まれています。

<span class="parent-field">network.vpc.</span><a id="network-vpc-id" href="#network-vpc-id" class="field">`id`</a> <span class="type">String</span>    
The ID of the VPC to import. This field is mutually exclusive with `cidr`.
インポートする VPC の ID。このフィールドは `cidr` とは排他的です。

<span class="parent-field">network.vpc.</span><a id="network-vpc-cidr" href="#network-vpc-cidr" class="field">`cidr`</a> <span class="type">String</span>    
Copilot が生成した VPC と関連付ける IPv4 CIDR ブロック。このフィールドは `id` とは排他的です。

<span class="parent-field">network.vpc.</span><a id="network-vpc-subnets" href="#network-vpc-subnets" class="field">`subnets`</a> <span class="type">Map</span>   
VPC にパブリックサブネットとプライベートサブネットを設定します。

例えば、既存の VPC をインポートする場合は、以下のようにします。
```yaml
network:
  vpc:
    id: 'vpc-12345'
    public:
      - id: 'subnet-11111'
      - id: 'subnet-22222'
```
または、Copilot で生成された VPC を構成する場合は、以下のようにします。
```yaml
network:
  vpc:
    cidr: '10.0.0.0/16'
    subnets:
      public:
        - cidr: '10.0.0.0/24'
          az: 'us-east-2a'
        - cidr: '10.0.1.0/24'
          az: 'us-east-2b'
```

<span class="parent-field">network.vpc.subnets.</span><a id="network-vpc-subnets-public" href="#network-vpc-subnets-public" class="field">`public`</a> <span class="type">Array of Subnets</span>    
パブリックサブネットに関する設定のリスト。

<span class="parent-field">network.vpc.subnets.</span><a id="network-vpc-subnets-private" href="#network-vpc-subnets-private" class="field">`private`</a> <span class="type">Array of Subnets</span>    
プライベートサブネットに関する設定のリスト。

<span class="parent-field">network.vpc.subnets.<type\>.</span><a id="network-vpc-subnets-id" href="#network-vpc-subnets-id" class="field">`id`</a> <span class="type">String</span>    
インポートするサブネットの ID。このフィールドは、`cidr` および `az` とは排他的です。

<span class="parent-field">network.vpc.subnets.<type\>.</span><a id="network-vpc-subnets-cidr" href="#network-vpc-subnets-cidr" class="field">`cidr`</a> <span class="type">String</span>    
サブネットに割り当てられた IPv4 CIDR ブロック。このフィールドは `id` とは排他的です。

<span class="parent-field">network.vpc.subnets.<type\>.</span><a id="network-vpc-subnets-az" href="#network-vpc-subnets-az" class="field">`az`</a> <span class="type">String</span>    
サブネットに割り当てられた可用性ゾーン名です。`az` フィールドはオプションです。デフォルトでは、Availability Zone はアルファベット順に割り当てられます。このフィールドは、`id` とは排他的です。

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-group" href="#network-vpc-security-group" class="field">`security_group`</a> <span class="type">Map</span>  
Environment のセキュリティグループに関するルール。
```yaml
network:
  vpc:
    security_group:
      ingress:
        - ip_protocol: tcp
          ports: 80  
          cidr: 0.0.0.0/0
```
<span class="parent-field">network.vpc.security_group.</span><a id="network-vpc-security-group-ingress" href="#network-vpc-security-group-ingress" class="field">`ingress`</a> <span class="type">Array of Security Group Rules</span>    
Environment のインバウンドセキュリティグループに関するルールのリスト。

<span class="parent-field">network.vpc.security-group.</span><a id="network-vpc-security-group-egress" href="#network-vpc-security-group-egress" class="field">`egress`</a> <span class="type">Array of Security Group Rules</span>    
Environment のアウトバウンドセキュリティグループに関するルールのリスト。

<span class="parent-field">network.vpc.security_group.<type\>.</span><a id="network-vpc-security-group-ip-protocol" href="#network-vpc-security-group-ip-protocol" class="field">`ip_protocol`</a> <span class="type">String</span>    
IP プロトコルの名前または番号。

<span class="parent-field">network.vpc.security_group.<type\>.</span><a id="network-vpc-security-group-ports" href="#network-vpc-security-group-ports" class="field">`ports`</a> <span class="type">String or Integer</span>     
セキュリティグループルールのポート範囲またはポート番号。

```yaml
ports: 0-65535
```

または

```yaml
ports: 80
```

<span class="parent-field">network.vpc.security_group.<type\>.</span><a id="network-vpc-security-group-cidr" href="#network-vpc-security-group-cidr" class="field">`cidr`</a> <span class="type">String</span>   
IPv4 アドレスの範囲を CIDR 形式で指定します。

<div class="separator"></div>

<a id="cdn" href="#cdn" class="field">`cdn`</a> <span class="type">Boolean or Map</span>  
The cdn section contains parameters related to integrating your service with a CloudFront distribution. To enable the CloudFront distribution, specify `cdn: true`.
cdn セクションには、CloudFront のディストリビューションと Service の統合に関連するパラメータが含まれています。CloudFront ディストリビューションを有効にするには、`cdn: true` と指定します。

<span class="parent-field">cdn.</span><a id="cdn-certificate" href="#cdn-certificate" class="field">`certificate`</a> <span class="type">String</span>  
CloudFront ディストリビューションで HTTPS トラフィックを有効にするための証明書。CloudFront は、インポートした証明書が `us-east-1` リージョンにあることが必須です。

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
http セクションには、[Load Balanced Web Service](./lb-web-service.ja.md) が共有するパブリックロードバランサーと、[Backend Service](./backend-service.ja.md) が共有する内部ロードバランサーを設定するためのパラメーターが含まれています。

<span class="parent-field">http.</span><a id="http-public" href="#http-public" class="field">`public`</a> <span class="type">Map</span>  
パブリックロードバランサーに関する設定。

<span class="parent-field">http.public.</span><a id="http-public-certificates" href="#http-public-certificates" class="field">`certificates`</a> <span class="type">Array of Strings</span>  
 [公開されている AWS Certificate Manager の証明書](https://docs.aws.amazon.com/ja_jp/acm/latest/userguide/gs-acm-request-public.html) ARN のリスト。
ロードバランサーにパブリック証明書をアタッチすることで、Load Balanced Web Service をドメイン名と関連付け、HTTPS で到達することができるようになります。[`http.alias`](./lb-web-service.ja.md#http-alias) を使用してサービスを再デプロイする方法の詳細については、[Developing/Domains](../developing/domain.ja.md#%E6%97%A2%E5%AD%98%E3%81%AE%E6%9C%89%E5%8A%B9%E3%81%AA%E8%A8%BC%E6%98%8E%E6%9B%B8%E3%81%AB%E5%90%AB%E3%81%BE%E3%82%8C%E3%82%8B%E3%83%89%E3%83%A1%E3%82%A4%E3%83%B3%E3%82%92%E4%BD%BF%E7%94%A8%E3%81%99%E3%82%8B) ガイドを参照してください。

<span class="parent-field">http.public.</span><a id="http-public-access-logs" href="#http-public-access-logs" class="field">`access_logs`</a> <span class="type">Boolean or Map</span>   
[Elastic Load Balancing のアクセスログ](https://docs.aws.amazon.com/ja_jp/elasticloadbalancing/latest/application/load-balancer-access-logs.html)を有効にします。
`true` を指定した場合、Copilot が S3 バケットを作成し、そこにパブリックロードバランサーがアクセスログを保存するようになります。

```yaml
http:
  public:
    access_logs: true 
```
ログのプレフィックスをカスタマイズすることができます。
```yaml
http:
  public:
    access_logs:
      prefix: access-logs
```

また、Copilot が作成した S3 バケットを使用するのではなく、自分で作成した S3 バケットを使用することも可能です。
```yaml
http:
  public:
    access_logs:
      bucket_name: my-bucket
      prefix: access-logs
```

<span class="parent-field">http.public.access_logs.</span><a id="http-public-access-logs-bucket-name" href="#http-public-access-logs-bucket-name" class="field">`bucket_name`</a> <span class="type">String</span>   
アクセスログの保存先となる既存の S3 バケット名。

<span class="parent-field">http.public.access_logs.</span><a id="http-public-access-logs-prefix" href="#http-public-access-logs-prefix" class="field">`prefix`</a> <span class="type">String</span>   
ログオブジェクトのプレフィックス。

<span class="parent-field">http.public.</span><a id="http-public-security-groups" href="#http-public-security-groups" class="field">`security_groups`</a> <span class="type">Map</span>    
パブリックロードバランサーに追加するセキュリティグループの設定。

<span class="parent-field">http.public.security_groups.</span><a id="http-public-security-groups-ingress" href="#http-public-security-groups-ingress" class="field">`ingress`</a> <span class="type">Map</span>  
パブリックロードバランサーを許可する Ingress ルール。  
```yaml
http:
  public:
    security_groups:
      ingress: 
        restrict_to:
          cdn: true
```

<span class="parent-field">http.public.security_groups.ingress.</span><a id="http-public-security-groups-ingress-restrict-to" href="#http-public-security-groups-ingress-restrict-to" class="field">`restrict_to`</a> <span class="type">Map</span>  
Public Load Balancer のトラフィックを制限する Ingress ルール。

<span class="parent-field">http.public.security_groups.ingress.restrict_to.</span><a id="http-public-security-groups-ingress-restrict-to-cdn" href="#http-public-security-groups-ingress-restrict-to-cdn" class="field">`cdn`</a> <span class="type">Boolean</span>    
パブリックロードバランサーの Ingress トラフィックが CloudFront ディストリビューションから来るように制限するかどうか。

<span class="parent-field">http.</span><a id="http-private" href="#http-private" class="field">`private`</a> <span class="type">Map</span>  
内部ロードバランサーの設定。

<span class="parent-field">http.private.</span><a id="http-private-certificates" href="#http-private-certificates" class="field">`certificates`</a> <span class="type">Array of Strings</span>  
[AWS Certificate Manager の証明書](https://docs.aws.amazon.com/ja_jp/acm/latest/userguide/gs.html) ARN のリスト。
ロードバランサーにパブリックまたはプライベート証明書をアタッチすることで、Backend Service をドメイン名と関連付け、HTTPS で到達することができます。[`http.alias`](./backend-service.ja.md#http-alias) を使用して Service を再デプロイする方法の詳細については、[Developing/Domains](../developing/domain.ja.md#%E6%97%A2%E5%AD%98%E3%81%AE%E6%9C%89%E5%8A%B9%E3%81%AA%E8%A8%BC%E6%98%8E%E6%9B%B8%E3%81%AB%E5%90%AB%E3%81%BE%E3%82%8C%E3%82%8B%E3%83%89%E3%83%A1%E3%82%A4%E3%83%B3%E3%82%92%E4%BD%BF%E7%94%A8%E3%81%99%E3%82%8B) ガイドを参照してください。

<span class="parent-field">http.private.</span><a id="http-private-subnets" href="#http-private-subnets" class="field">`subnets`</a> <span class="type">Array of Strings</span>   
内部ロードバランサーを配置するサブネット ID。

<span class="parent-field">http.private.</span><a id="http-private-security-groups" href="#http-private-security-groups" class="field">`security_groups`</a> <span class="type">Map</span>    
内部ロードバランサーに追加するセキュリティグループの設定。

<span class="parent-field">http.private.security_groups</span><a id="http-private-security-groups-ingress" href="#http-private-security-groups-ingress" class="field">`ingress`</a> <span class="type">Map</span>  
内部ロードバランサーを許可する Ingress ルール。  
```yaml
http:
  private:
    security_groups:
      ingress: # Enable incoming traffic within the VPC to the internal load balancer.
        from_vpc: true
```

<span class="parent-field">http.private.security_groups.ingress.</span><a id="http-private-security-groups-ingress-from-vpc" href="#http-private-security-groups-ingress-from-vpc" class="field">`from_vpc`</a> <span class="type">Boolean</span>    
VPC 内から内部ロードバランサーへのトラフィックを有効にするかどうか。

<div class="separator"></div>

<a id="observability" href="#observability" class="field">`observability`</a> <span class="type">Map</span>  
observability セクションでは、Environment にデプロイされた Service や Job に関するデータを収集する方法を設定します。

<span class="parent-field">observability.</span><a id="http-container-insights" href="#http-container-insights" class="field">`container_insights`</a> <span class="type">Bool</span>  
Environment の ECS クラスターで [CloudWatch の Container Insights](https://docs.aws.amazon.com/ja_jp/AmazonCloudWatch/latest/monitoring/ContainerInsights.html) を有効にするかどうか。
