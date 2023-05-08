# タスク定義のオーバーライド

!!! 注意
    :warning: タスク定義のオーバライドは非推奨です。 
    
    代わりに[YAML パッチ](./yamlpatch.ja.md) を利用することを推奨します。 YAML パッチは CloudFormation テンプレート全体の修正や、 削除操作もサポートしています。

Copilot は、[Manifest](../../manifest/overview.ja.md) で指定された構成を用いて CloudFormation テンプレートを生成します。ただし、Manifest では設定できないフィールドがあります。例えば、ワークロードにあるコンテナの [`Ulimits`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-containerdefinitions.html#cfn-ecs-taskdefinition-containerdefinition-ulimits) の設定を変更したいと思うかもしれませんが、Manifest では公開されていません。

`taskdef_overrides` ルールを指定することで、[ECS のタスク定義](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html)に追加設定することができます。このルールは、Copilot が Manifest から生成する CloudFormation テンプレートに適用されます。

## オーバーライドのルールを指定するには？

オーバーライドルールそれぞれに、オーバーライドしたい CloudFormation リソースフィールドの **path** とそのフィールドの **value** を指定する必要があります。

Manifest ファイルに適用可能な有効な `taskdef_overrides` フィールドの例を以下に示します。

``` yaml
taskdef_overrides:
- path: ContainerDefinitions[0].Cpu
  value: 512
- path: ContainerDefinitions[0].Memory
  value: 1024
```

それぞれのルールは CloudFormation テンプレートに順次適用されます。結果として得られた CloudFormation テンプレートが次のルールの対象となります。すべてのルールが正常に適用されるか、エラーが発生するまで評価が続けられます。

## path の評価

- `path` フィールドには、[CloudFormation におけるタスク定義の `Properties` 以下のフィールド](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html) を、`'.'` 区切りで入力します。

- CloudFormation のテンプレートにフィールドが存在しない場合、Copilot は再帰的にフィールドを挿入します。例えば、ルールが `A.B[-].C` というパスを持つ（`B` と `C` は存在しない）場合、Copilot は `B` と `C` というフィールドを挿入します。具体的な例としては、[以下](#add-ulimits-to-the-main-container)があります。

- ターゲットとなるパスが、既に存在するメンバーを指定している場合、そのメンバーの値が置き換えられます。

- `Ulimits` のような `list` フィールドに、新しいメンバーを追加するには特殊文字 `-` を使用します: `Ulimits[-]`。

!!! 注意

    タスク定義の以下のフィールドは変更ができません。

    * [Family](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html#cfn-ecs-taskdefinition-family)
    * [ContainerDefinitions[<index>].Name](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-containerdefinitions.html#cfn-ecs-taskdefinition-containerdefinition-name)

## テスト

オーバーライドルールが期待通りに動作することを確認するために、生成された CloudFormation テンプレートをプレビューします。`copilot svc package` または `copilot job package` を実行することをお勧めします。

## 例

### メインコンテナに `Ulimits` を追加

``` yaml
taskdef_overrides:
  - path: ContainerDefinitions[0].Ulimits[-]
    value:
      Name: "cpu"
      SoftLimit: 1024
      HardLimit: 2048
```

### 追加の UDP ポートを公開

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].PortMappings[-].ContainerPort"
    value: 2056
  # Copilot はデフォルトでポートマッピングを作成するため、PortMappings[1] とすることで、上記ルールで追加されたポートマッピングを取得します。
  - path: "ContainerDefinitions[0].PortMappings[1].Protocol"
    value: "udp"
```

### ルートファイルシステムを読み取り専用にする

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].ReadonlyRootFilesystem"
    value: true
```
