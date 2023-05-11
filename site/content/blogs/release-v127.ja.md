---
title: 'AWS Copilot v1.27: Copilot テンプレートの拡張、追加のルーティングルールのサポート、差分のプレビュー、サイドカーの拡張!'
twitter_title: 'AWS Copilot v1.27'
image: 'https://user-images.githubusercontent.com/879348/227655119-e42c6b8b-ff0e-4abe-ad90-89b44813fbd5.png'
image_alt: 'CDK overrides and diff'
image_width: '2056'
image_height: '1096'
---

# AWS Copilot v1.27: Copilot テンプレートの拡張、追加のルーティングルールのサポート、差分のプレビュー、サイドカーの拡張！
##### 投稿日: 2023 年 3 月 28 日

AWS Copilot コアチームは Copilot v1.27 のリリースを発表します 🚀。
私たちの公開[コミュニティチャット](https://gitter.im/aws/copilot-cli) は増え続けて、400 人以上がオンラインで、[GitHub](http://github.com/aws/copilot-cli/) には 2,700 以上のスターを獲得しています。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.27 は幾つかの新機能と改善点がある大きなリリースです。

- **Copilot テンプレートの拡張**: Copilot が生成した AWS CloudFormation テンプレートの任意のプロパティをカスタマイズできる様になりました。
AWS Cloud Development Kit (CDK) または YAML パッチオーバライドを使用します。[詳細セクションはこちらをご覧ください](#extend-copilot-generated-aws-cloudformation-templates)。
- **複数のリスナーとリスナールールが有効に**: [Application Load Balancer](../docs/manifest/lb-web-service.ja.md#http)にはホストベースまたはパスのリスナーのルールを、
[Network Load Balancer](../docs/manifest/lb-web-service.ja.md#nlb)に異なるポートやプロトコルのリスナーを複数定義できます。
[詳細セクションはこちらをご覧ください](#enable-multiple-listeners-and-listener-rules-for-load-balancers).
- **CloudFormation テンプレート変更のプレビュー**: `copilot [noun] package` または `copilot [noun] deploy` コマンドに `--diff` フラグを付けて実行すると、
最後にデプロイされた CloudFormation テンプレートとローカルの変更における差分を表示できる様になりました。[詳細セクションはこちらをご覧ください](#preview-aws-cloudformation-template-changes)。
- **サイドカーのコンテナイメージのビルドとプッシュ**: ローカルの Dockerfile からサイドカーコンテナをビルドしてプッシュするための `image.build` をサポートしました。[詳細セクションはこちらをご覧ください](#build-and-push-container-images-for-sidecar-containers)。
- **サイドカーに対する環境変数ファイルのサポート**: サイドカーコンテナ用のローカルな `.env` ファイルをプッシュするための `env_file` をサポートしました。[詳細セクションはこちらをご覧ください](#upload-local-environment-files-for-sidecar-containers)。

??? note "AWS Copilot とは？"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。
<a id="extend-copilot-generated-aws-cloudformation-templates"></a>
## Copilot が生成した AWS CloudFormation テンプレートの拡張

AWS Copilot を利用すると、`copilot init`　コマンドとプロンプトに従うだけで、ビルダーはコンテナ化されたアプリケーションを素早く開始できます。
その後、開発者は [Manifest](../docs/manifest/overview.ja.md) の infrastructure-as-code ファイルを編集し、デプロイすることで、アプリケーションを拡張できます。
さらに v1.27 では、 Copilot が生成した CloudFormation テンプレートを `copilot [noun] override` コマンドで拡張できるようになり、インフラストラクチャーを完全にカスタマイズできる様になりました。

<a id="aws-cloud-development-kit-cdk-overrides"></a>
### AWS Cloud Development Kit (CDK) オーバライド

プログラミング言語の表現力と安全性が必要な場合、 CDK を使用して CloudFormation テンプレートを拡張できます。
`copilot [noun] override` コマンドの実行後、 Copilot が `copilot/[name]/override` ディレクトリ配下に CDK アプリケーションを作成します。


```console
.
├── bin/
│   └── override.ts
├── .gitignore
├── cdk.json
├── package.json
├── README.md
├── stack.ts
└── tsconfig.json
```

プロパティの追加、削除、置換は、`stack.ts` ファイルを編集する、または `README.md` に記載された指示に従って行います。

??? note "View sample `stack.ts`"

    ```typescript
    import * as cdk from 'aws-cdk-lib';
    import * as path from 'path';
    import { aws_elasticloadbalancingv2 as elbv2 } from 'aws-cdk-lib';
    import { aws_ec2 as ec2 } from 'aws-cdk-lib';
    
    interface TransformedStackProps extends cdk.StackProps {
        readonly appName: string;
        readonly envName: string;
    }
    
    export class TransformedStack extends cdk.Stack {
        public readonly template: cdk.cloudformation_include.CfnInclude;
        public readonly appName: string;
        public readonly envName: string;
    
        constructor (scope: cdk.App, id: string, props: TransformedStackProps) {
            super(scope, id, props);
            this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
                templateFile: path.join('.build', 'in.yml'),
            });
            this.appName = props.appName;
            this.envName = props.envName;
            this.transformPublicNetworkLoadBalancer();
        }
    
        /**
         * transformPublicNetworkLoadBalancer removes the "Subnets" properties from the NLB,
         * and adds a SubnetMappings with predefined elastic IP addresses.
         */
        transformPublicNetworkLoadBalancer() {
            const elasticIPs = [new ec2.CfnEIP(this, 'ElasticIP1'), new ec2.CfnEIP(this, 'ElasticIP2')];
            const publicSubnets = cdk.Fn.importValue(`${this.appName}-${this.envName}-PublicSubnets`);
    
            // Apply the override.
            const nlb = this.template.getResource("PublicNetworkLoadBalancer") as elbv2.CfnLoadBalancer;
            nlb.addDeletionOverride('Properties.Subnets');
            nlb.subnetMappings = [{
                allocationId: elasticIPs[0].attrAllocationId,
                subnetId: cdk.Fn.select(0, cdk.Fn.split(",", publicSubnets)),
            }, {
                allocationId: elasticIPs[1].attrAllocationId,
                subnetId: cdk.Fn.select(1, cdk.Fn.split(",", publicSubnets)),
            }]
        }
    }
    ```

`copilot [noun] deploy` の実行や継続的デリバリーパイプラインがトリガーされると、Copilot はオーバライドされたテンプレートをデプロイします。
CDK による拡張について、さらに学ぶためには、[ガイド](../docs/developing/overrides/cdk.md)を確認してください。

<a id="yaml-patch-overrides"></a>
### YAML パッチオーバーライド

YAML パッチオーバーライドは、軽量なやり方です。
1) 他のツールやフレームワークと依存関係を持ちたく無い場合や、2) 少しの修正しか行わない場合に利用します。
`copilot [noun] override` コマンドを実行後、Copilot は 
`copilot/[name]/override` ディレクトリ配下にサンプルの `cfn.patches.yml`　を作成します。

```console
.
├── cfn.patches.yml
└── README.md
```

プロパティの追加、削除、置換は、`cfn.patches.yaml` ファイルを編集して行います。

??? note "View sample `cfn.patches.yml`"

    ```yaml
    - op: add
      path: /Mappings
      value:
        ContainerSettings:
          test: { Cpu: 256, Mem: 512 }
          prod: { Cpu: 1024, Mem: 1024}
    - op: remove
      path: /Resources/TaskRole
    - op: replace
      path: /Resources/TaskDefinition/Properties/ContainerDefinitions/1/Essential
      value: false
    - op: add
      path: /Resources/Service/Properties/ServiceConnectConfiguration/Services/0/ClientAliases/-
      value:
        Port: !Ref TargetPort
        DnsName: yamlpatchiscool
    ```

`copilot [noun] deploy` の実行や継続的デリバリーパイプラインがトリガーされると、Copilot はオーバライドされたテンプレートをデプロイします。
YAML パッチによる拡張について、さらに学ぶためには、[ガイド](../docs/developing/overrides/yamlpatch.md)を確認してください。

<a id="preview-aws-cloudformation-template-changes"></a>
## AWS CloudFormation テンプレート変更のプレビュー

##### `copilot [noun] package --diff`

`copilot [noun] package --diff` を実行すると、ローカルでの変更点と最新のデプロイ済みテンプレートとの差分を確認できるようになりました。
差分を表示したのちに、プログラムは終了します。

!!! info "`copilot [noun] package --diff` 実行時の終了コード"

    0 = no diffs found  
    1 = diffs found  
    2 = error-producing diffs


```console
$ copilot env deploy --diff
~ Resources:
    ~ Cluster:
        ~ Properties:
            ~ ClusterSettings:
                ~ - (changed item)
                  ~ Value: enabled -> disabled
```

差分に問題がなければ、再度 `copilot [noun] package` を実行し、テンプレートファイルとパラメータファイルを
指定したディレクトリに書き出します。


##### `copilot [noun] deploy --diff`

`copilot [noun] package --diff` と同様に、`copilot [noun] deploy --diff` で同じ差分を確認できます。
しかし、 Copilot は差分を表示した後に終了せず、`Continue with the deployment? [y/N]` と質問します。

```console
$ copilot job deploy --diff
~ Resources:
    ~ TaskDefinition:
        ~ Properties:
            ~ ContainerDefinitions:
                ~ - (changed item)
                  ~ Environment:
                      (4 unchanged items)
                      + - Name: LOG_LEVEL
                      +   Value: "info"

Continue with the deployment? (y/N)
```

差分に問題がなければ、"y" を入力してデプロイします。もしくは、"N" と入力して、必要に応じて調整します。

<a id="enable-multiple-listeners-and-listener-rules-for-load-balancers"></a>
## ロードバランサーに対する複数のリスナーとリスナールールが有効に
Application Load Balancer に追加のリスナールールを設定できるようになりました。同様に、
Network Load Balancer にも追加のリスナーが設定できます。
<a id="add-multiple-host-based-or-path-based-listener-rules-to-your-application-load-balancer"></a>
### Application Load Balancer に対する複数のホストベースまはたパスベースのリスナールールの追加
新しいフィールド [`http.additional_rules`](../docs/manifest/lb-web-service.ja.md#http-additional-rules) を利用して、ALB に追加のリスナールールを設定できます。
設定例を通じて確認しましょう。

Service がパス `customerdb`、 `admin` 、 `superadmin` を異なるコンテナポートでトラフィックを取り扱いたいとします。
```yaml
name: 'frontend'
type: 'Load Balanced Web Service'
 
image:
  build: Dockerfile
  port: 8080
  
http:
  path: '/'
  additional_rules:            # The new field "additional_rules".
    - path: 'customerdb'  
      target_port: 8081        # Optional. Defaults to the `image.port`.
    - path: 'admin'
      target_container: nginx   # Optional. Defaults to the main container. 
      target_port: 8082
    - path: 'superAdmin'   
      target_port: 80

sidecars:
  nginx:
    port: 80
    image: public.ecr.aws/nginx:latest
```
この Manifest では、“/” へのリクエストは、メインコンテナのポート 8080 へルーティングされます。"/customerdb" へのリクエストは、メインコンテナのポート 8081 にルーティングされます。
"/admin" へのリクエストは、nginx のポート 8082 へ、"/superAdmin"へのリクエストは nginx のポート 80 にルーティングされます。なお、3 つ目のリスナールールは 'target_port: 80' と定義されています。
つまり、Copilot は '/superAdmin' からのトラフィックを賢く nginx サイドカーコンテナへルーティングします。

また、"/" へのリクエストを処理するコンテナポートを新しいフィールド[`http.target_port`](../docs/manifest/lb-web-service.ja.md#http-target-port)でも設定可能です。

<a id="add-multiple-port-and-protocol-listeners-to-your-network-load-balancers"></a>
### Network Load Balancer に対する 複数のポートやプロコトルのリスナー追加
新しいフィールド [`nlb.additional_listeners`](../docs/manifest/lb-web-service.ja.md#nlb-additional-listeners)を利用して、NLB に対する追加のリスナーを設定できます。
設定例を通じて確認しましょう。

```yaml
name: 'frontend'
type: 'Load Balanced Web Service'

image:
  build: Dockerfile

http: false
nlb:
  port: 8080/tcp
  additional_listeners:
    - port: 8081/tcp
    - port: 8082/tcp
      target_port: 8085               # Optional. Default is set 8082.
      target_container: nginx         # Optional. Default is set to the main container.

sidecars:
  nginx:
    port: 80
    image: public.ecr.aws/nginx:latest
```
この Manifest では、NLB ポート 8080 へのリクエストはメインコンテナのポート 8080 にルーティングされます。
NLB ポート 8081 へのリクエストは、メインコンテナのポート 8081 にルーティングされます。
ここで注意しなければならないのは、target_port のデフォルト値は対応する NLB ポートの値と同じであることです。
NLB ポート 8082 へのリクエストは nginx というサイドカーコンテナのポート 8085 にルーティングされます。 

<a id="sidecar-improvements"></a>
## サイドカーの改善

メインコンテナと同じ様に、サイドカーコンテナに対してもコンテナイメージのビルドとプッシュができるようになりました。
加えて、サイドカーに対するローカルの環境変数ファイルのパスを指定できる様になっています。
<a id="build-and-push-container-images-for-sidecar-containers"></a>
### サイドカーコンテナに対する コンテナイメージのビルドとプッシュ

Copilot では、Dockerfile からネイティブにサイドカーコンテナイメージを構築し、ECR へプッシュできるようになりました。
この機能を利用するためには、ユーザーはいくつかの方法でワークロード Manifest を変更します。

最初のオプションは、Dockerfile へのパスを単純に文字列として指定する事です。

```yaml
sidecars:
  nginx:
    image:
      build: path/to/dockerfile
```

また、 `build` フィールドを Map として指定すると、より高度なカスタマイズが行えます。
これには、Dockerfile パスの指定やコンテキストディレクトリ、 ターゲットするビルドステージ、 イメージからのキャッシュ、ビルド引数が含まれます。

```yaml
sidecars:
  nginx:
    image:
      build:
        dockerfile: path/to/dockerfile
        context: context/dir
        target: build-stage
        cache_from:
          - image: tag
        args: value
```

他のオプションとして、Dockerfile からビルドする代わりに、既存のイメージ URI を指定する方法もあります。

```yaml
sidecars:
  nginx:
    image: 123457839156.dkr.ecr.us-west-2.amazonaws.com/demo/front:nginx-latest
```

また、location フィールドを利用して、イメージ URI を指定できます。
```yaml
sidecars:
  nginx:
    image:
      location:  123457839156.dkr.ecr.us-west-2.amazonaws.com/demo/front:nginx-latest
```
<a id="upload-local-environment-files-for-sidecar-containers"></a>
### サイドカーコンテナ用のローカルの環境変数ファイルをアップロードする
タスク内の任意のサイドカーにアップロードする環境変数ファイルを指定できる様になりました。
以前は、 Task のメインコンテナに対する環境変数ファイルのみが指定できました。

```yaml
# in copilot/{service name}/manifest.yml
env_file: log.env
```

これからは、サイドカー定義において、同じことが行えます。
```yaml
sidecars:
  nginx:
    image: nginx:latest
    env_file: ./nginx.env
    port: 8080
```

マネージドな `logging` サイドカーに対しても利用できます。

```yaml
logging:
  retention: 1
  destination:
    Name: cloudwatch
    region: us-west-2
    log_group_name: /copilot/logs/
    log_stream_prefix: copilot/



  env_file: ./logging.env
```

異なるサイドカーで同じファイルを複数回していした場合、Copilot はファイルを 1 回だけ S3 にアップロードします。
