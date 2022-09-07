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

## Ingress 規制とは?

特定の送信元からの受信トラフィックを制限することができます。CloudFront の場合、Copilot は [AWS マネージドプレフィックスリスト](https://docs.aws.amazon.com/ja_jp/vpc/latest/userguide/working-with-aws-managed-prefix-lists.html)を使用して、CloudFront エッジロケーションに関連する CIDR IP アドレスのセットに許可されたトラフィックを制限します。`restrict_to.cdn: true` を指定すると、パブリックロードバランサーは完全にパブリックアクセスできなくなり、CloudFront 配信からのみアクセスできるようになり、Service に対するセキュリティ脅威から保護されます。