# CDK オーバーライド

{% include 'overrides-intro.ja.md' %}

## YAML パッチよりも CDK オーバーライドを使用するのはどの様な場合ですか？

どちらのオプションも Copilot [Manifest](../../manifest/overview.ja.md) によって表面化されない機能にアクセスして設定するという"ガラスを壊す"仕組みです。

プログラミング言語の表現力が必要な場合、[YAML パッチ](./yamlpatch.ja.md)よりも AWS Cloud Development Kit (CDK) オーバーライドをお勧めします。
CDK を使えば、CloudFormation テンプレートに安全かつ強力な修正が行えます。

## 始め方

`copilot [noun] override` コマンドを実行すると、CDK　を使って CloudFormation テンプレートを拡張できます。
例えば、`copilot svc override` コマンドにより、 Load Balanced Web Service のテンプレートを更新します。

コマンドは、以下の様な構造で、`copilot/[name]/override` 配下に新しい CDK アプリケーションを作成します。
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

`stack.ts` を編集するとこから開始します。例えば、  `copilot svc override` を利用して、
ECS サービスのプロパティをオーバーライドする場合、次の様な `stack.ts` ファイルが生成されので、修正をします。

```typescript
import * as cdk from 'aws-cdk-lib';
import { aws_ecs as ecs } from 'aws-cdk-lib';

export class TransformedStack extends cdk.Stack {
    constructor (scope: cdk.App, id: string, props?: cdk.StackProps) {
        super(scope, id, props);
         this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
            templateFile: path.join('.build', 'in.yaml'),
        });
        this.appName = template.getParameter('AppName').valueAsString;
        this.envName = template.getParameter('EnvName').valueAsString;

        this.transformService();
    }
 
    // TODO: implement me.
    transformService() {
      const service = this.template.getResource("Service") as ecs.CfnService;
      throw new error("not implemented");
    }
}
```

## どの様な仕組みでしょうか？

上記の `stack.ts` ファイルを見ればわかるとおり、Copilot は [cloudformation_include module](https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.cloudformation_include-readme.html) を使用します。これは、CDK が提供している、変換を支援する為のライブラリです。
このライブラリは ["Import or migrate an existing AWS CloudFormation template"](https://docs.aws.amazon.com/cdk/v2/guide/use_cfn_template.html) ガイドで推奨されています。
このライブラリにより Copilot Manifest によって表示されないリソースに対して、 [L1 constructs](https://docs.aws.amazon.com/cdk/v2/guide/constructs.html) としてアクセスできる様になります。
`CfnInclude` オブジェクトは、隠された `.build/in.yaml` CloudFormation テンプレートから初期化されます。
これが Copilot と CDK のコミュニケーション方法です。
Copilot は `.build/` ディレクトリ配下に Manifest より生成された CloudFormation テンプレートを出力します。それを `cloudformation_include` ライブラリで解析し、CDK コンストラクトに変換します。

`copilot [noun] package` または `copilot [noun] deploy` を実行するたびに、Copilot はまず Manifest ファイルから CloudFormation テンプレートを作成します。それを CDK アプリケーションに渡して、プロパティをオーバーライドします。

デプロイの前に CDK の変更を確認するために、`package` または `deploy` コマンドで `--diff` フラグを使用することを強くお勧めします。

## 実行例

次の例は Elastic IP アドレスを Network Load Balancer　へ割り当てる為に Load Balanced Web Service の [`nlb`](../../manifest/lb-web-service.ja.md#nlb) リソースを修正しています。

この例で、以下の方法を確認できます。

- リソースのプロパティ削除
- 新しいリソースの作成
- 既存リソースのプロパティの修正

??? note "`stack.ts` のサンプルを見る"

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

次の例では、 プロダクション環境など特定の環境だけのためにプロパティを追加する方法を紹介します。

??? note "`stack.ts` のサンプルを見る"

    ```typescript
    import * as cdk from 'aws-cdk-lib';
    import * as path from 'path';
    import { aws_iam as iam } from 'aws-cdk-lib';
    
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
            this.transformEnvironmentManagerRole();
        }
        
        transformEnvironmentManagerRole() {
            const environmentManagerRole = this.template.getResource("EnvironmentManagerRole") as iam.CfnRole;
            if (this.envName === "prod") {
                let assumeRolePolicy = environmentManagerRole.assumeRolePolicyDocument
                let statements = assumeRolePolicy.Statement
                statements.push({
                     "Effect": "Allow",
                     "Principal": { "Service": "ec2.amazonaws.com" },
                     "Action": "sts:AssumeRole"
                })
            }
        }
    }
    ```

次の例ではリソースを削除する方法を示しています。具体的には Copilot が作成し Service のログを保持するデフォルトのロググループです。

??? note "`stack.ts` のサンプルを見る"

    ```typescript
    import * as cdk from 'aws-cdk-lib';
    import * as path from 'path';

    interface TransformedStackProps extends cdk.StackProps {
        readonly appName: string;
        readonly envName: string;
    }

    export class TransformedStack extends cdk.Stack {
        public readonly template: cdk.cloudformation_include.CfnInclude;
        public readonly appName: string;
        public readonly envName: string;

        constructor(scope: cdk.App, id: string, props: TransformedStackProps) {
            super(scope, id, props);
            this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
            templateFile: path.join('.build', 'in.yml'),
            });
            this.appName = props.appName;
            this.envName = props.envName;
            // Deletes the default log group resource.
            this.template.node.tryRemoveChild("LogGroup")
        }
    }
    ```
