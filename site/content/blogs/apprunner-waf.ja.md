# Copilot における App Runner での WAF の利用

投稿日 : 2023 年 2 月 23 日

**Siddharth Vohra**, ソフトウエア ディベロップメント エンジニア, AWS App Runner

AWS Web Application Firewall (WAF) のウェブアクセスコントロールリスト (ウェブ ACL) を App Runner サービスに関連付けることができるようになりました。 - Copilot で！

[AWS Web Application Firewall (WAF)](https://docs.aws.amazon.com/ja_jp/waf/latest/developerguide/waf-chapter.html) は、Web アプリケーションに転送される HTTP(S) リクエストの監視を支援し、
コンテンツへのアクセスを制御します。
指定した基準に基づいて、Web リクエストを拒否したり、許可することでアプリケーションを保護できます。
例えば、 リクエストの発信元 IP アドレスやクエリ文字列の値などでアクセスを制御できます。

AWS　は [AWS App Runner がセキュリティ強化のためのウェブアプリケーションファイアウォール (WAF) サポートを導入](https://aws.amazon.com/jp/about-aws/whats-new/2023/02/aws-app-runner-web-application-firewall-enhanced-security/)を発表しました。
この発表は[Request-Driven Web Services](../docs/concepts/services.ja.md#request-driven-web-service) を AWS WAF で保護できるようになったことを意味します。
このブログポストでは、AWS Copilot を利用して、簡単に保護を有効にする方法を紹介します。



!!!info
    これらの手順は、[GitHub "Show and tell" discussion section](https://github.com/aws/copilot-cli/discussions/4542) に投稿したものと同じです！もし App Runner の WAF サポートについて、質問、フィードバック、リクエストがあれば、気軽にコメントを残してください！


### 前提条件
続行するには、独自の WAF ウェブアクセスコントロールリスト(ウェブ ACL) を用意する必要があります。
まだ、ACL を作成していない場合、まずルールオプションでアプリケーション用の
WAF ACL を作成する必要があります(独自のウェブ ACL の作成については [こちら](https://docs.aws.amazon.com/waf/latest/developerguide/web-acl-creating.html)を確認してください)。

ウェブ ACL が準備できたら、その ARN をメモしておきます。
WAF ウェブ ACL を Request-Driven Web Service で利用するには、 次の手順に従います。

### 手順 1 (オプション): Request-Driven Web Service の作成
Request-Driven Web Service がまだ無い場合、
次のコマンドを実行して、 App Runner サービスを作成・設定します。
```console
copilot init \
  --svc-type "Request-Driven Web Service" \
  --name "waf-example" \
  --dockerfile path/to/Dockerfile
```
または、`copilot svc init` をフラグ無しで実行します。Copilot は追加の入力を求めるプロンプトを表示し、
Service の作成をガイドします。

### 手順 2 (オプション): Service に対応した  `addons/` フォルダの作成

Request-Driven Web Service があるワークスペースにいるとします。手順 1 を実行している場合、
ワークスペースの構造は次の様になっているでしょう。
```term
.
└── copilot/
  └── waf-example/ # The name of your Request-Driven Web Service. Not necessarily "waf-example".
      └── manifest.yml
```

`./copilot/<name of your Request-Driven Web Service>` 配下に、`addons/`　フォルダがまだ無い場合、作成します。
ワークスペースは次の様になっているでしょう。
```term
.
└── copilot/
  └── waf-example/ # The name of your Request-Driven Web Service. Not necessarily "waf-example".
      ├── manifest.yml
      └── addons/
```

### 手順 3: Addon を利用して、Request-Driven Web Service に ウェブ ACL を関連づける 

Addon フォルダに、 `waf.yml` と `addons.parameters.yml` という 2 つのファイルを新しく作成します。フォルダは次の様になっているでしょう。

  ```term
  .
  └── copilot
      └── waf-example/ # The name of your Request-Driven Web Service. Not necessarily "waf-example".
          ├── manifest.yml
          └── addons/
              ├── waf.yml 
              └── addons.parameters.yml
  ```

次の内容をコピーし、それぞれのファイルに貼り付けます。

=== "waf.yml"
    ```yaml
    #Addon template to add WAF configuration to your App Runner service.
    
    Parameters:
      App:
        Type: String
        Description: Your application's name.
      Env:
        Type: String
        Description: The environment name your service, job, or workflow is being deployed to.
      Name:
        Type: String
        Description: The name of the service, job, or workflow being deployed.
      ServiceARN:
        Type: String
        Default: ""
        Description: The ARN of the service being deployed.
    
    Resources:
      # Configuration of the WAF Web ACL you want to asscoiate with 
      # your App Runner service.
      Firewall:
        Metadata:
          'aws:copilot:description': 'Associating your App Runner service with your WAF WebACL'
        Type: AWS::WAFv2::WebACLAssociation
        Properties: 
          ResourceArn: !Sub ${ServiceARN}
          WebACLArn:  <paste your WAF Web ACL ARN here> # Paste your WAF Web ACL ARN here.
    ```

=== "addons.parameters.yml"  
      ```yaml
      Parameters:
        ServiceARN: !Ref Service
      ```


### 手順 4: ウェブ ACL ARN を `waf.yml` に入力する

`waf.yml` を開き、`<paste your WAF Web ACL ARN here>` をウェブ ACL リソースの ARN で置き換えます。例えば、次の部分です。

```yaml
Resources:
  # Configuration of the WAF Web ACL you want to associate with 
  # your App Runner service
  Firewall:
    Metadata:
      'aws:copilot:description': 'Associating your App Runner service with your WAF WebACL'
    Type: AWS::WAFv2::WebACLAssociation
    Properties: 
      ResourceArn: !Sub ${ServiceARN}
      WebACLArn: arn:aws:wafv2:us-east-2:123456789138:regional/webacl/mytestwebacl/3df43564-be9f-47ce-a12b-3a577d2d8913
```
 

### 手順 5: Service をデプロイする
最後に、`copilot svc deploy` を実行します！　Request-Driven Web Service がデプロイされ、 WAF ウェブ ACL に関連付けられているでしょう！

???+ note "考慮事項"
    - ウェブ ACL は複数の Services に関連付けられますが、1 つの Service には 1 つより多くのウェブ ACL を関連付けることができません。
    - Copilot を利用した App Runner サービスが既にある場合、 手順 2-5 を実行するだけで、 WAF ウェブ ACL を既存の Service に関連づけることができます。
