# グローバルなコンテンツ配信

Copilot は、Amazon CloudFront を介した Content Delivery Network をサポートしています。このリソースは Copilot によって Environment レベルで管理され、ユーザーは [Environment Manifest](../manifest/environment.ja.md) を通じて CloudFront を活用することができます。

## Copilot による CloudFront インフラストラクチャ

Copilot が [CloudFront ディストリビューション](https://docs.aws.amazon.com/ja_jp/AmazonCloudFront/latest/DeveloperGuide/distribution-overview.html) を作成すると、Application Load Balancer の代わりに Application への新しいエントリポイントになるようディストリビューションが作成されます。これにより、CloudFront は世界中に配置されたエッジロケーションを経由して、ロードバランサーにトラフィックをより速くルーティングすることができます。

## 既存のアプリケーションで CloudFront を使うには?

Copilot v1.20 から、`copilot env init` で Environment Manifest ファイルが作成されるようになりました。この Manifest に `cdn: true` という値を指定し、`copilot env deploy` を実行すると、基本的な CloudFront ディストリビューションを有効にすることができます。

???+ note "CloudFront ディストリビューション Manifest の設定例"

    === "Basic"

        ```yaml
        cdn: true

        http:
          public:
            security_groups:
              ingress:
                restrict_to:
                  cdn: true
        ```
    
    === "Imported Certificates"

        ```yaml
        cdn:
          certificate: arn:aws:acm:us-east-1:${AWS_ACCOUNT_ID}:certificate/13245665-h74x-4ore-jdnz-avs87dl11jd

        http:
          certificates:
            - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/13245665-bldz-0an1-afki-p7ll1myafd
            - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/56654321-cv8f-adf3-j7gd-adf876af95
        ```

## CloudFront で HTTPS トラフィックを有効にするには?

CloudFront で HTTPS を使用する場合、ロードバランサーの `http.certificates` フィールドと同じように、Environment Manifest の `cdn.certificate` フィールドで証明書を指定します。ロードバランサーとは異なり、インポートできる証明書は 1 つだけです。このため、その Environment で Service が使用する各エイリアスを検証するために、CNAME レコードを使用して新しい証明書 (リージョンは `us-east-1`) を作成することをお勧めします。

!!! info
    CloudFront は、`us-east-1` リージョンでインポートされた証明書のみをサポートしています。

!!! info
    CloudFront 用の証明書をインポートすると、Environment Manager ロールに権限が追加され、Copilot が `DescribeCertificate` [API call](https://docs.aws.amazon.com/ja_jp/acm/latest/APIReference/API_DescribeCertificate.html) を使用できるようになります。

また、Application の作成時に `--domain` を指定することで、Copilot に証明書の管理を任せることができます。この場合、CloudFront 対応 Environment に展開するすべての Service に `http.alias` を指定する必要があります。

この 2 つの設定により、Copilot は CloudFront が [SSL/TLS 証明書](https://docs.aws.amazon.com/ja_jp/AmazonCloudFront/latest/DeveloperGuide/using-https-alternate-domain-names.html)を使用するように設定されます。これにより、ビューワーの証明書を検証し、HTTPS 接続を有効にすることができます。

## Ingress 制限とは?

特定の送信元からの受信トラフィックを制限できます。CloudFront の場合、Copilot は [AWS マネージドプレフィックスリスト](https://docs.aws.amazon.com/ja_jp/vpc/latest/userguide/working-with-aws-managed-prefix-lists.html)を使用して、CloudFront エッジロケーションに関連する CIDR IP アドレスのセットに許可されたトラフィックを制限します。`restrict_to.cdn: true` を指定すると、パブリックロードバランサーはパブリックアクセスできなくなり、CloudFront ディストリビューションからのみアクセスできるようになり、Service に対するセキュリティ脅威から保護されます。

## CloudFront で TLS を終端させるには？

!!! attention
    1. Load Balanced Web Service の [HTTP to HTTPS redirection](../../manifest/lb-web-service/#http-redirect-to-https) を無効化します。
    2. CloudFront TLS ターミネーションを有効にする前に、全ての Load Balanced Web Service 対して、個別に `svc deploy` を実行して再デプロイを行います。
    3. 全ての Load Balanced Web Service が HTTP を HTTPS にリダイレクトしなくなったら、Environment Manifest 内で CloudFront TLS ターミネーションを安全に有効化できます。`env deploy` を実行します。


Environment Manifest を次の様に設定することで、オプションで CloudFront を TLS ターミネーションに使用できます。

```yaml
cdn:
  terminate_tls: true
```

`CloudFront → Application Load Balancer (ALB) → ECS` の通信は、HTTP のみになります。エンドユーザに地理的に近いエンドポイントで TLS を終端させる事で、 TLS ハンドシェイクを高速化させる利点があります。

## S3 バケットで CloudFront を使うには？
Environment Manifest で `cdn.static_assets` を設定することで、CloudFront が Amazon S3 バケットと連携し、静的コンテンツを高速に配信することも可能です。

### 既存の S3 バケットの利用

!!! attention
    セキュリティの観点から、**プライベート**な S3 バケットを使用し、デフォルトですべてのパブリックアクセスがブロックされるようにすることをお勧めします。

以下の Environment Manifest の例では、既存の S3 バケットを CloudFront 用に使用する方法を説明しています。

???+ note "既存の S3 バケットを CloudFront で使用するための Environment Manifest 設定例"
    ```yaml
    cdn:
      static_assets:
        location: cf-s3-ecs-demo-bucket.s3.us-west-2.amazonaws.com
        alias: example.com
        path: static/*
    ```

`static_assets.location` は S3 バケットの DNS ドメイン名 (例えば、`EXAMPLE-BUCKET.s3.us-west-2.amazonaws.com` など) にします。[Application に関連するルートドメイン](./domain.ja.md#application-に関連するルートドメインを使用する)のエイリアスを使用していない場合は、CloudFront のドメイン名を指すエイリアスの A レコードを忘れずに作成してください。

Environment Manifest で Environment をデプロイした後は、(プライベートバケットの場合) S3 バケットのバケットポリシーを更新して、CloudFront がアクセスできるようにする必要があります。

???+ note "CloudFront に読み取り専用アクセスを許可する S3 バケットポリシーの例"
    ```json
    {
        "Version": "2012-10-17",
        "Statement": {
            "Sid": "AllowCloudFrontServicePrincipalReadOnly",
            "Effect": "Allow",
            "Principal": {
                "Service": "cloudfront.amazonaws.com"
            },
            "Action": "s3:GetObject",
            "Resource": "arn:aws:s3:::EXAMPLE-BUCKET/*",
            "Condition": {
                "StringEquals": {
                    "AWS:SourceArn": "arn:aws:cloudfront::111122223333:distribution/EDFDVBD6EXAMPLE"
                }
            }
        }
    }
    ```

## CloudFront を使用して静的な Web サイトを提供するには？
[copilot init](../commands/init.ja.md) または [copilot svc init](../commands/svc-init.ja.md) コマンドを使用して、Static Site ワークロードを作成します。アップロードするファイルを選択すると、Copilot は個別の専用 CloudFront ディストリビューションと、アセットを含む S3 バケットをプロビジョニングします。再デプロイのたびに、よりダイナミックな開発のために、Copilot は既存のキャッシュを無効化します。