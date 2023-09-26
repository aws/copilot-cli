!!! Attention

    :warning: CloudFormation テンプレートのオーバーライドは高度な機能であり、スタックのデプロイが正常にデプロイされない原因となる可能性があります。
    注意して使用してください！

Copilot は [Manifest](../../manifest/overview.ja.md) で指定した設定を使用して CloudFormation テンプレートを生成します。
一方で、 Manifest では、全ての CloudFormation プロパティが設定可能となってはいません。
例えば、ワークロードコンテナのために、[`Ulimits`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-ulimit.html) を
設定したい場合がありますが、そのプロパティは Manifest では公開されていません。 

`yamlpatch` や `cdk` によるオーバーライドでは、CloudFormation テンプレートの_任意_のプロパティやリソースを追加、削除、置換できます。
